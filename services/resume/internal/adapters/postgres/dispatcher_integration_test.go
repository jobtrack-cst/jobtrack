//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	rabbit "github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func dispatcherBroker(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "publisher_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
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
	if err != nil {
		t.Fatal("get RabbitMQ host failed")
	}
	port, err := container.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatal("get RabbitMQ port failed")
	}
	brokerURL := (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("publisher_test", "synthetic-test-password"), Path: "/"}).String()
	return brokerURL
}

type dispatchPublishFunc func(context.Context, *eventsv1.EventEnvelope) error

func (f dispatchPublishFunc) Publish(ctx context.Context, e *eventsv1.EventEnvelope) error {
	return f(ctx, e)
}

type failedCompletion struct{ ports.OutboxRepository }

func (f failedCompletion) MarkPublished(context.Context, string, string) (bool, error) {
	return false, errors.New("simulated database outage after confirmation")
}

func TestOutboxDispatcher_IntegrationDeliveryAndRecovery(t *testing.T) {
	ctx, pool := startResumeDB(t)
	broker := dispatcherBroker(t, ctx)
	require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	repo := postgres.NewOutbox(pool)
	publisher := func() *rabbit.Publisher {
		t.Helper()
		p, err := rabbit.NewPublisher(broker)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, p.Close()) })
		return p
	}
	isPublished := func(id string) bool {
		t.Helper()
		var published *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT published_at FROM outgoing_events WHERE event_id=$1`, id).Scan(&published))
		return published != nil
	}
	take := func(count int, id string) {
		t.Helper()
		q, err := ch.QueueDeclarePassive("resume-validator.events", true, false, false, false, nil)
		require.NoError(t, err)
		require.Zero(t, q.Consumers)
		for i := 0; i < count; i++ {
			msg, ok, err := ch.Get(q.Name, false)
			require.NoError(t, err)
			require.True(t, ok)
			event, err := events.UnmarshalEnvelope(msg.Body)
			require.NoError(t, err)
			require.Equal(t, id, event.EventId)
			require.NoError(t, msg.Ack(false))
		}
		_, ok, err := ch.Get(q.Name, false)
		require.NoError(t, err)
		require.False(t, ok)
	}
	t.Run("confirmation precedes database completion and no consumer is required", func(t *testing.T) {
		id, body := seedOutgoing(t, ctx, pool, base)
		p := publisher()
		wrapped := dispatchPublishFunc(func(c context.Context, e *eventsv1.EventEnvelope) error {
			require.False(t, isPublished(id))
			require.NoError(t, p.Publish(c, e)) // Real broker confirmation.
			require.False(t, isPublished(id))   // Completion is still forbidden until return.
			require.Equal(t, id, e.EventId)
			return nil
		})
		d := app.NewOutboxDispatcher(repo, wrapped, nil)
		delay, err := d.Step(ctx)
		require.NoError(t, err)
		require.Zero(t, delay)
		require.True(t, isPublished(id))
		take(1, id)
		var stored string
		require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM outgoing_events WHERE event_id=$1`, id).Scan(&stored))
		require.Equal(t, body, stored)
	})
	for _, scenario := range []string{"closed transport", "uncertain confirmation", "confirmed but completion failed", "canceled after acceptance"} {
		t.Run(scenario, func(t *testing.T) {
			id, body := seedOutgoing(t, ctx, pool, base)
			p := publisher()
			var output ports.EventPublisher = p
			var store ports.OutboxRepository = repo
			expectedDeliveries := 1
			attemptCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			if scenario == "closed transport" {
				require.NoError(t, p.Close())
			}
			if scenario == "uncertain confirmation" {
				expectedDeliveries = 2
				output = dispatchPublishFunc(func(c context.Context, e *eventsv1.EventEnvelope) error {
					require.NoError(t, p.Publish(c, e))
					return errors.New("simulated lost confirmation")
				})
			}
			if scenario == "confirmed but completion failed" {
				expectedDeliveries = 2
				store = failedCompletion{repo}
			}
			if scenario == "canceled after acceptance" {
				expectedDeliveries = 2
				output = dispatchPublishFunc(func(c context.Context, e *eventsv1.EventEnvelope) error {
					require.NoError(t, p.Publish(c, e))
					cancel()
					return context.Canceled
				})
			}
			_, err := app.NewOutboxDispatcher(store, output, nil).Step(attemptCtx)
			require.Error(t, err)
			require.False(t, isPublished(id))
			var scheduled *time.Time
			require.NoError(t, pool.QueryRow(ctx, `SELECT next_attempt_at FROM outgoing_events WHERE event_id=$1`, id).Scan(&scheduled))
			if scenario == "closed transport" || scenario == "uncertain confirmation" {
				require.NotNil(t, scheduled)
			}
			// Advance persisted eligibility relative to database time; no wall-clock sleeps.
			_, err = pool.Exec(ctx, `UPDATE outgoing_events SET lease_until=CASE WHEN claim_token IS NULL THEN NULL ELSE statement_timestamp()-INTERVAL '1 second' END, next_attempt_at=statement_timestamp()-INTERVAL '1 second' WHERE event_id=$1`, id)
			require.NoError(t, err)
			// New repository, worker and publisher model a process restart.
			_, err = app.NewOutboxDispatcher(postgres.NewOutbox(pool), publisher(), nil).Step(ctx)
			require.NoError(t, err)
			require.True(t, isPublished(id))
			take(expectedDeliveries, id)
			var stored string
			require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM outgoing_events WHERE event_id=$1`, id).Scan(&stored))
			require.Equal(t, body, stored)
		})
	}
	t.Run("concurrent workers and expired ownership cannot complete another claim", func(t *testing.T) {
		first, _ := seedOutgoing(t, ctx, pool, base)
		second, _ := seedOutgoing(t, ctx, pool, base.Add(time.Second))
		p1, p2 := publisher(), publisher()
		entered := make(chan struct{})
		release := make(chan struct{})
		done := make(chan error, 1)
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		blocked := dispatchPublishFunc(func(c context.Context, e *eventsv1.EventEnvelope) error {
			close(entered)
			select {
			case <-release:
				return p1.Publish(c, e)
			case <-c.Done():
				return c.Err()
			}
		})
		go func() { _, err := app.NewOutboxDispatcher(repo, blocked, nil).Step(ctx); done <- err }()
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("first worker did not claim")
		}
		var oldToken string
		require.NoError(t, pool.QueryRow(ctx, `SELECT claim_token FROM outgoing_events WHERE event_id=$1`, first).Scan(&oldToken))
		_, err := app.NewOutboxDispatcher(postgres.NewOutbox(pool), p2, nil).Step(ctx)
		require.NoError(t, err)
		require.True(t, isPublished(second))
		require.False(t, isPublished(first))
		_, err = pool.Exec(ctx, `UPDATE outgoing_events SET lease_until=statement_timestamp()-INTERVAL '1 second' WHERE event_id=$1`, first)
		require.NoError(t, err)
		_, err = app.NewOutboxDispatcher(postgres.NewOutbox(pool), p2, nil).Step(ctx)
		require.NoError(t, err)
		require.True(t, isPublished(first))
		close(release)
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("stale worker did not return")
		}
		changed, err := repo.Retry(ctx, first, oldToken, 0)
		require.NoError(t, err)
		require.False(t, changed)
		counts := map[string]int{}
		for i := 0; i < 3; i++ {
			msg, ok, err := ch.Get("resume-validator.events", false)
			require.NoError(t, err)
			require.True(t, ok)
			e, err := events.UnmarshalEnvelope(msg.Body)
			require.NoError(t, err)
			counts[e.EventId]++
			require.NoError(t, msg.Ack(false))
		}
		require.Equal(t, map[string]int{first: 2, second: 1}, counts)
	})
	t.Run("idle worker cancellation", func(t *testing.T) {
		workerCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		p := publisher()
		go func() { done <- app.NewOutboxDispatcher(repo, p, nil).Run(workerCtx) }()
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("idle worker did not stop")
		}
	})
}

func TestOutboxDispatcher_IntegrationMalformedDoesNotStarve(t *testing.T) {
	ctx, pool := startResumeDB(t)
	broker := dispatcherBroker(t, ctx)
	require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
	publisher, err := rabbit.NewPublisher(broker)
	require.NoError(t, err)
	defer publisher.Close()
	older, _ := seedOutgoing(t, ctx, pool, base)
	newer, _ := seedOutgoing(t, ctx, pool, base.Add(time.Second))
	// Simulate persisted corruption while preserving the schema's envelope ID/type.
	malformed := `{"event_id":"` + older + `","type":"resume.uploaded"}`
	_, err = pool.Exec(ctx, `UPDATE outgoing_events SET payload=$2 WHERE event_id=$1`, older, malformed)
	require.NoError(t, err)
	d := app.NewOutboxDispatcher(postgres.NewOutbox(pool), publisher, nil)
	wait, err := d.Step(ctx)
	require.Error(t, err)
	require.Zero(t, wait, "persisted event retry must not pause all claims")
	var pending bool
	var scheduled bool
	var token *string
	var lease *time.Time
	var stored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT published_at IS NULL, next_attempt_at > statement_timestamp(), claim_token, lease_until, payload FROM outgoing_events WHERE event_id=$1`, older).Scan(&pending, &scheduled, &token, &lease, &stored))
	require.True(t, pending)
	require.True(t, scheduled)
	require.Nil(t, token)
	require.Nil(t, lease)
	require.Equal(t, malformed, stored)
	wait, err = d.Step(ctx)
	require.NoError(t, err)
	require.Zero(t, wait)
	var published bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT published_at IS NOT NULL FROM outgoing_events WHERE event_id=$1`, newer).Scan(&published))
	require.True(t, published)
	require.NoError(t, pool.QueryRow(ctx, `SELECT published_at IS NULL, next_attempt_at > statement_timestamp(), payload FROM outgoing_events WHERE event_id=$1`, older).Scan(&pending, &scheduled, &stored))
	require.True(t, pending)
	require.True(t, scheduled)
	require.Equal(t, malformed, stored)
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	msg, ok, err := ch.Get("resume-validator.events", false)
	require.NoError(t, err)
	require.True(t, ok)
	envelope, err := events.UnmarshalEnvelope(msg.Body)
	require.NoError(t, err)
	require.Equal(t, newer, envelope.EventId)
	require.NoError(t, msg.Ack(false))
	_, ok, err = ch.Get("resume-validator.events", false)
	require.NoError(t, err)
	require.False(t, ok, "malformed event must never be published")
	wait, err = d.Step(ctx)
	require.NoError(t, err)
	require.Equal(t, app.OutboxIdleWait, wait, "no eligible events must use idle wait")
}
