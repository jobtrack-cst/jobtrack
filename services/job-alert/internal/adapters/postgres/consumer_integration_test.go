//go:build integration

package postgres_test

import (
	"context"
	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestJobConsumer_BrokerDuplicatesInvalidAndShutdown(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	broker := startAlertsBroker(t, ctx)
	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/job.published.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	var job eventsv1.JobPublished
	require.NoError(t, e.Data.UnmarshalTo(&job))
	_, err = repo.Create(ctx, syntheticAlert(uuid.NewString(), job.Location))
	require.NoError(t, err)
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	ready := make(chan struct{}, 1)
	cfg := rabbitmq.JobConsumerConfig(broker, slog.New(slog.NewTextHandler(io.Discard, nil)), app.NewJobs(postgres.NewJobs(pool), integrationClock{}))
	cfg.OnReady = func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	go func() { done <- events.Consume(workerCtx, cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("consumer shutdown timed out")
		}
	})
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	pub, err := events.NewPublisher(broker)
	require.NoError(t, err)
	defer pub.Close()
	require.NoError(t, pub.Publish(ctx, e))
	require.NoError(t, pub.Publish(ctx, e))
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	bad := proto.Clone(e).(*eventsv1.EventEnvelope)
	bad.EventId = uuid.NewString()
	bad.Producer = "offer"
	require.NoError(t, pub.Publish(ctx, bad))
	require.Eventually(t, func() bool {
		q, err := ch.QueueDeclarePassive("job-alert.jobs.dead", true, false, false, false, nil)
		return err == nil && q.Messages == 1
	}, 10*time.Second, 50*time.Millisecond)
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events").Scan(&count))
	require.Equal(t, 1, count)
}
