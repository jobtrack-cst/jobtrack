//go:build integration

package postgres_test

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
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

// The decision, the outbox and the relay together: an approved and a rejected
// request each reach RabbitMQ once, with their routing key, after the commit.
func TestOutboxRelay_IntegrationPublishesCommittedDecisions(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	brokerURL := startRelayBroker(t, ctx)
	repo, outbox := postgres.NewRequests(pool), postgres.NewOutbox(pool)

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
	for _, key := range []string{app.EventEmployerVerified, app.EventEmployerVerificationRejected} {
		require.NoError(t, ch.QueueBind(q.Name, key, "jobtrack.events", false, nil))
	}
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)

	approved := createAndApprove(t, ctx, repo, 1, employerID, base.Add(time.Hour))
	approvedEvent := eventFor(t, approved)
	_, err = repo.Decide(ctx, approved, approvedEvent)
	require.NoError(t, err)

	rejected := newPending(2, otherID, base)
	_, err = repo.Create(ctx, rejected)
	require.NoError(t, err)
	require.NoError(t, rejected.Reject(adminID, "Expired licence", base.Add(2*time.Hour)))
	rejectedEvent := eventFor(t, rejected)
	_, err = repo.Decide(ctx, rejected, rejectedEvent)
	require.NoError(t, err)

	relay := app.NewOutboxRelay(outbox, publisher, fixedClock{at: base.Add(3 * time.Hour)})
	n, err := relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	for _, want := range []struct {
		key   string
		event proto.Message
	}{{app.EventEmployerVerified, approvedEvent}, {app.EventEmployerVerificationRejected, rejectedEvent}} {
		select {
		case d := <-deliveries:
			require.Equal(t, want.key, d.RoutingKey, "oldest decision first")
			got, err := events.UnmarshalEnvelope(d.Body)
			require.NoError(t, err)
			require.True(t, proto.Equal(want.event, got))
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
