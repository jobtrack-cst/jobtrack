//go:build integration

package rabbitmq_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

const (
	alBadEventID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	alAdminID    = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

func alStartPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine",
		pgcontainer.WithDatabase("audit_log_test"), pgcontainer.WithUsername("audit_log_test"),
		pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := db.OpenPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")))
	return pool
}

func alStartRabbitMQ(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "consumer_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
			WaitingFor: wait.ForListeningPort("5672/tcp"),
		},
		Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
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
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()),
		User: url.UserPassword("consumer_test", "synthetic-test-password"), Path: "/"}).String()
}

func alQueueMessages(t *testing.T, brokerURL, queue string) int {
	t.Helper()
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
	require.NoError(t, err)
	return q.Messages
}

// alPublishRaw skips the shared publisher, so an envelope it might refuse
// still reaches the consumer.
func alPublishRaw(t *testing.T, ctx context.Context, brokerURL, routingKey string, body []byte) {
	t.Helper()
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.PublishWithContext(ctx, "jobtrack.events", routingKey, false, false,
		amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body}))
}

// alInvalidJobClosed is the job.closed fixture with a producer that fails
// the JT-35 kebab-case rule and its own event_id.
func alInvalidJobClosed(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixturesDir, "job.closed.json"))
	require.NoError(t, err)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(raw, &envelope))
	for _, key := range []string{"eventId", "event_id"} {
		if _, ok := envelope[key]; ok {
			envelope[key] = alBadEventID
		}
	}
	envelope["producer"] = "Job_Posting"
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	return body
}

func alWaitFor(t *testing.T, ctx context.Context, what string, check func() bool) {
	t.Helper()
	for !check() {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestEventsConsumer_IntegrationRecordsEveryEventOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := alStartPostgres(t, ctx)
	brokerURL := alStartRabbitMQ(t, ctx)
	auditLog := app.NewAuditLog(postgres.NewEntries(pool))

	cfg := rabbitmq.EventsConsumerConfig(brokerURL, slog.New(slog.NewTextHandler(io.Discard, nil)), auditLog)
	ready := make(chan struct{}, 1)
	cfg.OnReady = func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	consumeCtx, stopConsumer := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- events.Consume(consumeCtx, cfg) }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("consumer stopped before it was ready: %v", err)
	case <-ctx.Done():
		t.Fatal("consumer did not become ready")
	}

	publisher, err := events.NewPublisher(brokerURL)
	require.NoError(t, err)
	defer publisher.Close()
	jobClosed := loadFixture(t, "job.closed.json")
	// Two routing keys from different producers reach the one "#" queue; the
	// second job.closed has the same event_id and must not add an entry.
	for _, e := range []*eventsv1.EventEnvelope{jobClosed, loadFixture(t, "job.closed.json"), loadFixture(t, "employer.verified.json")} {
		require.NoError(t, publisher.Publish(ctx, e))
	}
	alPublishRaw(t, ctx, brokerURL, "job.closed", alInvalidJobClosed(t))

	admin := domain.Caller{UserID: alAdminID, Role: domain.RoleAdmin}
	list := func() app.EntryPage {
		page, err := auditLog.ListEntries(ctx, admin, app.ListEntriesInput{})
		require.NoError(t, err)
		return page
	}
	alWaitFor(t, ctx, "both events to be recorded", func() bool { return list().TotalCount == 2 })
	alWaitFor(t, ctx, "the invalid event in the dead queue", func() bool {
		return alQueueMessages(t, brokerURL, rabbitmq.EventsQueue+".dead") == 1
	})
	// Give the duplicate time to be handled; it must not add an entry.
	time.Sleep(time.Second)
	page := list()
	require.EqualValues(t, 2, page.TotalCount, "the duplicate event_id is ignored")

	var found bool
	for _, e := range page.Entries {
		require.NotEqual(t, alBadEventID, e.EventID, "the invalid event is never recorded")
		if e.EventID == strings.ToLower(jobClosed.GetEventId()) {
			found = true
			require.Equal(t, "job.closed", e.EventType)
			require.Equal(t, jobClosed.GetProducer(), e.Producer)
			require.True(t, jobClosed.GetOccurredAt().AsTime().Equal(e.OccurredAt))
			require.Contains(t, e.DataJSON, "@type")
		}
	}
	require.True(t, found, "the job.closed fixture is listed for an admin")

	stopConsumer()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop")
	}
	require.Zero(t, alQueueMessages(t, brokerURL, rabbitmq.EventsQueue), "nothing is left in the queue")
	require.Equal(t, 1, alQueueMessages(t, brokerURL, rabbitmq.EventsQueue+".dead"), "only the invalid event is dead-lettered")
}
