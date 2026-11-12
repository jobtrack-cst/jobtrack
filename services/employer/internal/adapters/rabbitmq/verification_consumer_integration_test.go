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

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

type utcClock struct{}

func (utcClock) Now() time.Time { return time.Now().UTC() }

func startConsumerPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("employer_test"), pgcontainer.WithUsername("employer_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func startConsumerRabbitMQ(t *testing.T, ctx context.Context) string {
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

func countProcessed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events`).Scan(&n))
	return n
}

// waitProcessed polls until the consumer has handled want events.
func waitProcessed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	for countProcessed(t, ctx, pool) < want {
		select {
		case <-ctx.Done():
			t.Fatalf("consumer processed %d of %d events", countProcessed(t, ctx, pool), want)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestVerificationConsumer_IntegrationAppliesSharedFixtures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := startConsumerPostgres(t, ctx)
	brokerURL := startConsumerRabbitMQ(t, ctx)
	profiles := postgres.NewProfiles(pool)
	_, err := profiles.Create(ctx, domain.NewCompanyProfile(fixtureEmployerID,
		domain.ProfileFields{CompanyName: "Druk Tech"}, time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC)))
	require.NoError(t, err)

	verifications := app.NewVerifications(postgres.NewVerifications(pool), utcClock{})
	cfg := rabbitmq.VerificationConsumerConfig(brokerURL, slog.New(slog.NewTextHandler(io.Discard, nil)), verifications)
	ready := make(chan struct{}, 1)
	cfg.OnReady = func() { ready <- struct{}{} }
	consumeCtx, stopConsumer := context.WithCancel(ctx)
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

	// The newer approval arrives first, then the older rejection, then the
	// approval again: the profile must end up verified.
	verified := loadFixture(t, "employer.verified.json")
	for _, e := range []*eventsv1.EventEnvelope{verified, loadFixture(t, "employer.verification_rejected.json"), verified} {
		require.NoError(t, publisher.Publish(ctx, e))
	}
	waitProcessed(t, ctx, pool, 2)
	// Give the duplicate time to be handled; it must not add a row.
	time.Sleep(time.Second)
	require.Equal(t, 2, countProcessed(t, ctx, pool), "the duplicate event_id is ignored")

	got, err := profiles.GetByEmployerID(ctx, fixtureEmployerID)
	require.NoError(t, err)
	require.Equal(t, domain.VerificationVerified, got.VerificationStatus)
	require.Empty(t, got.RejectionReason)

	stopConsumer()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop")
	}

	// Nothing was dead-lettered and nothing is left in the queue.
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	for _, queue := range []string{rabbitmq.VerificationQueue, rabbitmq.VerificationQueue + ".dead"} {
		q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
		require.NoError(t, err)
		require.Zero(t, q.Messages, queue)
	}
}
