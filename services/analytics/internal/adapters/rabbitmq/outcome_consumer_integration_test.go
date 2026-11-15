//go:build integration

package rabbitmq_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

func startOutcomePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("analytics_test"), pgcontainer.WithUsername("analytics_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func startOutcomeRabbitMQ(t *testing.T, ctx context.Context) string {
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
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("consumer_test", "synthetic-test-password"), Path: "/"}).String()
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query).Scan(&n))
	return n
}

func queueMessages(t *testing.T, ch *amqp.Channel, queue string) int {
	t.Helper()
	q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
	require.NoError(t, err)
	return q.Messages
}

func waitFor(t *testing.T, ctx context.Context, what string, done func() bool) {
	t.Helper()
	for !done() {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestOutcomeConsumer_IntegrationRecordsSharedFixtures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := startOutcomePostgres(t, ctx)
	brokerURL := startOutcomeRabbitMQ(t, ctx)
	analytics := app.NewAnalytics(postgres.NewOutcomes(pool))

	cfg := rabbitmq.OutcomeConsumerConfig(brokerURL, slog.New(slog.NewTextHandler(io.Discard, nil)), analytics)
	ready := make(chan struct{}, 1)
	// Non-blocking, because OnReady runs again after every reconnect.
	cfg.OnReady = func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	consumeCtx, stopConsumer := context.WithCancel(ctx)
	defer stopConsumer()
	done := make(chan error, 1)
	go func() { done <- events.Consume(consumeCtx, cfg) }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("consumer did not become ready")
	}

	publisher, err := events.NewPublisher(brokerURL)
	require.NoError(t, err)
	defer publisher.Close()

	keys := rabbitmq.OutcomeBindings()
	for _, key := range keys {
		require.NoError(t, publisher.Publish(ctx, loadFixture(t, key)))
	}
	// A redelivered event, then one with the wrong producer. The consumer
	// handles one event at a time, so the duplicate is done before the
	// invalid event reaches the dead queue.
	require.NoError(t, publisher.Publish(ctx, loadFixture(t, "job.published")))
	invalid := loadFixture(t, "job.published")
	invalid.EventId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1"
	invalid.Producer = "application"
	require.NoError(t, publisher.Publish(ctx, invalid))

	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	const outcomes, processed = `SELECT count(*) FROM outcome_events`, `SELECT count(*) FROM processed_events`
	waitFor(t, ctx, "the eight fixtures to be stored", func() bool { return countRows(t, ctx, pool, outcomes) == len(keys) })
	waitFor(t, ctx, "the invalid event in the dead queue", func() bool { return queueMessages(t, ch, rabbitmq.OutcomeQueue+".dead") == 1 })
	require.Equal(t, len(keys), countRows(t, ctx, pool, outcomes), "the duplicate adds no row")
	require.Equal(t, len(keys), countRows(t, ctx, pool, processed), "the invalid event is not marked processed")

	// The stored events change the admin summary.
	start, end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	admin := domain.Caller{UserID: "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f", Role: domain.RoleAdmin}
	summary, err := analytics.GetSummary(ctx, admin, app.SummaryInput{Start: &start, End: &end})
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.JobsPublished)
	require.Len(t, summary.StatusChanges, 1)
	require.Equal(t, "UNDER_REVIEW", summary.StatusChanges[0].Status)
	require.EqualValues(t, 1, summary.StatusChanges[0].Count)

	r, err := domain.NewTimeRange(&start, &end)
	require.NoError(t, err)
	counts, err := postgres.NewOutcomes(pool).CountEvents(ctx, r)
	require.NoError(t, err)
	require.Len(t, counts, len(keys), "one row per event type")
	for _, c := range counts {
		require.EqualValues(t, 1, c.Count, c.EventType)
	}

	stopConsumer()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop")
	}
	require.Zero(t, queueMessages(t, ch, rabbitmq.OutcomeQueue), "nothing is left in the source queue")
}
