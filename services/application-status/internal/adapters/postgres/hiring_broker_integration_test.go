//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

// Both consumers on a real broker and database: the shared fixtures move the
// fixture application from SUBMITTED to ACCEPTED, redeliveries change nothing,
// an event from a final state is dead-lettered, and the relay publishes every
// application.status_changed in order.
func TestHiringEvents_RealBrokerAndPostgres(t *testing.T) {
	ctx, pool := startStatusDB(t)
	url := startRelayBroker(t, ctx)
	repo, out := postgres.NewStatuses(pool), postgres.NewOutbox(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 2)
	for _, cfg := range []events.ConsumerConfig{
		rabbitmq.ApplicationConsumerConfig(url, logger, repo),
		rabbitmq.HiringConsumerConfig(url, logger, repo),
	} {
		ready := make(chan struct{})
		cfg.OnReady = func() {
			select {
			case <-ready:
			default:
				close(ready)
			}
		}
		go func() { done <- events.Consume(workerCtx, cfg) }()
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal("consumer never became ready", cfg.Queue)
		}
	}
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
		require.NoError(t, <-done)
	})

	pub, err := rabbitmq.NewPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	conn, err := amqp.Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	require.NoError(t, err)
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "application.status_changed", "jobtrack.events", false, nil))
	received, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)

	fixture := func(name string) *eventsv1.EventEnvelope {
		raw, err := os.ReadFile("../../../../../tests/contract/fixtures/" + name + ".json")
		require.NoError(t, err)
		e, err := events.UnmarshalEnvelope(raw)
		require.NoError(t, err)
		return e
	}
	appID := "11111111-1111-4111-8111-111111111111"
	waitFor := func(want domain.Status) {
		t.Helper()
		require.Eventually(t, func() bool {
			s, err := repo.Get(ctx, appID)
			return err == nil && s.Status == want
		}, 15*time.Second, 50*time.Millisecond, "status %s", want)
	}

	require.NoError(t, pub.Publish(ctx, fixture("application.submitted")))
	waitFor(domain.StatusSubmitted)
	for _, step := range []struct {
		name string
		want domain.Status
	}{
		{"review.completed", domain.StatusShortlisted},
		{"interview.scheduled", domain.StatusInterview},
		{"offer.made", domain.StatusOffered},
		{"offer.accepted", domain.StatusAccepted},
	} {
		e := fixture(step.name)
		require.NoError(t, pub.Publish(ctx, e))
		require.NoError(t, pub.Publish(ctx, e), "redelivery")
		waitFor(step.want)
	}

	declined := fixture("offer.declined")
	require.NoError(t, pub.Publish(ctx, declined))
	require.Eventually(t, func() bool {
		dead, err := ch.QueueDeclarePassive(rabbitmq.HiringQueue+".dead", true, false, false, false, nil)
		return err == nil && dead.Messages == 1
	}, 15*time.Second, 100*time.Millisecond, "offer.declined after ACCEPTED is dead-lettered")
	assert.Equal(t, domain.StatusAccepted, func() domain.Status { s, _ := repo.Get(ctx, appID); return s.Status }())

	assert.Equal(t, []string{"submitted", "under_review", "shortlisted", "interview", "offered", "accepted"}, history(t, ctx, pool, appID))
	relay := app.NewOutboxRelay(out, pub, time.Now)
	n, err := relay.PublishPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	var got []string
	for range 6 {
		select {
		case d := <-received:
			e, err := events.UnmarshalEnvelope(d.Body)
			require.NoError(t, err)
			assert.Equal(t, "application-status", e.GetProducer())
			var data eventsv1.StatusChanged
			require.NoError(t, e.GetData().UnmarshalTo(&data))
			got = append(got, data.GetPreviousStatus()+">"+data.GetNewStatus())
		case <-time.After(10 * time.Second):
			t.Fatal("application.status_changed was not delivered")
		}
	}
	assert.Equal(t, []string{"UNSPECIFIED>SUBMITTED", "SUBMITTED>UNDER_REVIEW", "UNDER_REVIEW>SHORTLISTED",
		"SHORTLISTED>INTERVIEW", "INTERVIEW>OFFERED", "OFFERED>ACCEPTED"}, got)
}
