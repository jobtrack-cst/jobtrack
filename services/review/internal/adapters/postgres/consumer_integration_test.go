//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

// End to end on a real broker and database: application.submitted in through
// review.applications, review.completed out through the outbox relay.
func TestReviewEvents_RealBrokerAndPostgres(t *testing.T) {
	ctx, pool := startReviewDB(t)
	brokerURL := startReviewBroker(t, ctx)

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	ready := make(chan struct{})
	var readyOnce sync.Once
	cfg := rabbitmq.ApplicationConsumerConfig(brokerURL, slog.New(slog.NewTextHandler(io.Discard, nil)), postgres.NewParticipants(pool))
	cfg.OnReady = func() { readyOnce.Do(func() { close(ready) }) }
	go func() { done <- events.Consume(workerCtx, cfg) }()
	t.Cleanup(func() { cancel(); require.NoError(t, <-done) })
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("application consumer did not become ready", ctx.Err())
	}

	pub, err := rabbitmq.NewPublisher(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	// A temporary queue sees what review publishes.
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	require.NoError(t, err)
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "review.completed", "jobtrack.events", false, nil))
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)

	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	submitted, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)

	t.Run("application.submitted is recorded once, even when delivered twice", func(t *testing.T) {
		require.NoError(t, pub.Publish(ctx, submitted))
		require.NoError(t, pub.Publish(ctx, submitted))
		require.Eventually(t, func() bool {
			return count(t, ctx, pool, `SELECT count(*) FROM processed_events WHERE event_id = $1`, submitted.GetEventId()) == 1
		}, 10*time.Second, 50*time.Millisecond)
		p, err := postgres.NewParticipants(pool).GetByApplicationID(ctx, "11111111-1111-4111-8111-111111111111")
		require.NoError(t, err)
		assert.Equal(t, "22222222-2222-4222-8222-222222222222", p.ApplicantID)
		assert.Equal(t, "55555555-5555-4555-8555-555555555555", p.EmployerID)
	})

	t.Run("conflicting participants go to the dead queue", func(t *testing.T) {
		var data eventsv1.ApplicationSubmitted
		require.NoError(t, submitted.GetData().UnmarshalTo(&data))
		data.EmployerId = uuid.NewString()
		packed, err := anypb.New(&data)
		require.NoError(t, err)
		conflicting := proto.Clone(submitted).(*eventsv1.EventEnvelope)
		conflicting.EventId, conflicting.Data = uuid.NewString(), packed
		require.NoError(t, pub.Publish(ctx, conflicting))
		require.Eventually(t, func() bool {
			dead, err := ch.QueueDeclarePassive(rabbitmq.ApplicationQueue+".dead", true, false, false, false, nil)
			return err == nil && dead.Messages == 1
		}, 10*time.Second, 100*time.Millisecond)
		assert.Zero(t, count(t, ctx, pool, `SELECT count(*) FROM processed_events WHERE event_id = $1`, conflicting.GetEventId()))
		p, err := postgres.NewParticipants(pool).GetByApplicationID(ctx, data.GetApplicationId())
		require.NoError(t, err)
		assert.Equal(t, "55555555-5555-4555-8555-555555555555", p.EmployerID, "stored employer unchanged")
	})

	t.Run("a completed review is published as review.completed", func(t *testing.T) {
		p, err := postgres.NewParticipants(pool).GetByApplicationID(ctx, "11111111-1111-4111-8111-111111111111")
		require.NoError(t, err)
		rv := reviewFor(p, 4, domain.DecisionShortlisted)
		require.NoError(t, postgres.NewReviews(pool).Create(ctx, rv))
		relay := app.NewOutboxRelay(postgres.NewOutbox(pool), pub, utcClock{})
		n, err := relay.PublishPending(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		select {
		case d := <-deliveries:
			assert.Equal(t, "review.completed", d.RoutingKey)
			e, err := events.UnmarshalEnvelope(d.Body)
			require.NoError(t, err)
			assert.Equal(t, "review", e.GetProducer())
			assert.Equal(t, rv.CreatedAt, e.GetOccurredAt().AsTime())
			var data eventsv1.ReviewCompleted
			require.NoError(t, e.GetData().UnmarshalTo(&data))
			assert.Equal(t, p.ApplicationID, data.GetApplicationId())
			assert.Equal(t, "SHORTLISTED", data.GetDecision())
		case <-time.After(10 * time.Second):
			t.Fatal("review.completed was not delivered")
		}
		n, err = relay.PublishPending(ctx)
		require.NoError(t, err)
		assert.Zero(t, n, "nothing is published twice")
	})
}
