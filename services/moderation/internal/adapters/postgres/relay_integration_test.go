//go:build integration

package postgres_test

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func startRelayBroker(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "relay_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
			WaitingFor: wait.ForListeningPort("5672/tcp"),
		},
		Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate RabbitMQ test container failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start RabbitMQ test container failed")
	}
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("relay_test", "synthetic-test-password"), Path: "/"}).String()
}

// The use case, the outbox and the relay together: two takedowns each reach
// RabbitMQ once, oldest first, with the routing key and producer that
// job-posting's consumer (JT-68) checks, and only after the commit.
func TestOutboxRelay_IntegrationPublishesCommittedRemovals(t *testing.T) {
	ctx, pool := startModerationDB(t)
	brokerURL := startRelayBroker(t, ctx)
	outbox := postgres.NewOutbox(pool)

	publisher, err := rabbitmq.NewPublisher(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })

	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	require.NoError(t, err)
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, app.EventJobRemoved, "jobtrack.events", false, nil))
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)

	// Remove through the real use case, so the stored event is the one
	// production builds.
	clock := &steppingClock{times: []time.Time{base.Add(time.Hour), base.Add(2 * time.Hour)}}
	reports := app.NewReports(postgres.NewReports(pool), clock)
	admin := domain.Caller{UserID: adminID, Role: domain.RoleAdmin}
	for _, job := range []string{jobA, jobB} {
		_, err := reports.RemoveJob(ctx, admin, job, "fake company")
		require.NoError(t, err)
	}

	relay := app.NewOutboxRelay(outbox, publisher, fixedClock{at: base.Add(3 * time.Hour)})
	n, err := relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	for _, wantJob := range []string{jobA, jobB} {
		select {
		case d := <-deliveries:
			require.Equal(t, "moderation.job_removed", d.RoutingKey)
			got, err := events.UnmarshalEnvelope(d.Body)
			require.NoError(t, err)
			require.Equal(t, "moderation", got.GetProducer())
			var data eventsv1.ModerationJobRemoved
			require.NoError(t, got.GetData().UnmarshalTo(&data))
			require.Equal(t, wantJob, data.GetJobId(), "oldest removal first")
			require.Equal(t, "fake company", data.GetReason())
			stored := storedEvent(t, ctx, pool, got.GetEventId())
			require.True(t, proto.Equal(stored, got), "the broker receives exactly the stored envelope")
		case <-ctx.Done():
			t.Fatal("timed out waiting for the published event")
		}
	}

	// Published events are marked, so the next run sends nothing again.
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	select {
	case d := <-deliveries:
		t.Fatalf("unexpected second delivery %s", d.RoutingKey)
	case <-time.After(500 * time.Millisecond):
	}
}

// steppingClock returns its times in order, one per call.
type steppingClock struct{ times []time.Time }

func (c *steppingClock) Now() time.Time {
	at := c.times[0]
	if len(c.times) > 1 {
		c.times = c.times[1:]
	}
	return at
}

// storedEvent reads one outbox payload back as an envelope.
func storedEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) *eventsv1.EventEnvelope {
	t.Helper()
	var payload string
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM outbox_events WHERE event_id = $1`, id).Scan(&payload))
	e, err := events.UnmarshalEnvelope([]byte(payload))
	require.NoError(t, err)
	return e
}
