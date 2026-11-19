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
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

const (
	jcOtherJobID = "44444444-4444-4444-8444-444444444444"
	jcApplicantA = "a1111111-1111-4111-8111-111111111111"
	jcApplicantB = "b2222222-2222-4222-8222-222222222222"
	jcBadEventID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

func jcStartPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine",
		pgcontainer.WithDatabase("saved_jobs_test"), pgcontainer.WithUsername("saved_jobs_test"),
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

func jcStartRabbitMQ(t *testing.T, ctx context.Context) string {
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

func jcCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&n))
	return n
}

func jcQueueMessages(t *testing.T, brokerURL, queue string) int {
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

// jcWaitFor polls until check is true or the context ends.
func jcWaitFor(t *testing.T, ctx context.Context, what string, check func() bool) {
	t.Helper()
	for !check() {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestJobClosedConsumer_IntegrationDeletesBookmarksOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := jcStartPostgres(t, ctx)
	brokerURL := jcStartRabbitMQ(t, ctx)
	repo := postgres.NewSavedJobsRepository(pool)
	for _, s := range []struct{ job, applicant string }{
		{fixtureClosedJobID, jcApplicantA}, {fixtureClosedJobID, jcApplicantB}, {jcOtherJobID, jcApplicantA},
	} {
		require.NoError(t, repo.SaveJob(ctx, s.job, s.applicant))
	}

	cfg := rabbitmq.JobClosedConsumerConfig(brokerURL, slog.New(slog.NewTextHandler(io.Discard, nil)), app.NewSavedJobs(repo))
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
	bad := loadJobClosedFixture(t)
	bad.EventId, bad.Producer = jcBadEventID, "saved-jobs"
	for _, e := range []*eventsv1.EventEnvelope{loadJobClosedFixture(t), loadJobClosedFixture(t), bad} {
		require.NoError(t, publisher.Publish(ctx, e))
	}

	jcWaitFor(t, ctx, "the fixture to be processed", func() bool {
		return jcCount(t, ctx, pool, "SELECT count(*) FROM processed_events") == 1
	})
	jcWaitFor(t, ctx, "the bad event in the dead queue", func() bool {
		return jcQueueMessages(t, brokerURL, rabbitmq.JobsQueue+".dead") == 1
	})
	// Give the duplicate time to be handled; it must not add a row.
	time.Sleep(time.Second)
	require.Equal(t, 1, jcCount(t, ctx, pool, "SELECT count(*) FROM processed_events"), "the duplicate event_id is ignored")
	require.Zero(t, jcCount(t, ctx, pool, "SELECT count(*) FROM saved_jobs WHERE job_id = $1", fixtureClosedJobID))
	require.Equal(t, 1, jcCount(t, ctx, pool, "SELECT count(*) FROM saved_jobs WHERE job_id = $1", jcOtherJobID))
	require.ErrorIs(t, repo.SaveJob(ctx, fixtureClosedJobID, jcApplicantA), domain.ErrJobClosed)

	stopConsumer()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop")
	}
	require.Zero(t, jcQueueMessages(t, brokerURL, rabbitmq.JobsQueue), "nothing is left in the queue")
	require.Equal(t, 1, jcQueueMessages(t, brokerURL, rabbitmq.JobsQueue+".dead"), "only the bad event is dead-lettered")
}
