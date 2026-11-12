//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
)

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

func eventFixture(t *testing.T, kind string) *eventsv1.EventEnvelope {
	t.Helper()
	b, err := os.ReadFile("../../../../../tests/contract/fixtures/application." + kind + ".json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	return e
}
func input(t *testing.T, kind string) domain.ApplicationEvent {
	e, err := rabbitmq.ApplicationFromEnvelope(eventFixture(t, kind))
	require.NoError(t, err)
	return e
}
func counts(t *testing.T, ctx context.Context, repo *postgres.Outbox, want int) {
	t.Helper()
	pending, err := repo.Pending(ctx, 1000)
	require.NoError(t, err)
	require.Len(t, pending, want)
}

func TestApplicationEvents_AtomicIdempotentAndConcurrent(t *testing.T) {
	ctx, pool := startStatusDB(t)
	repo := postgres.NewStatuses(pool)
	out := postgres.NewOutbox(pool)
	submitted := input(t, "submitted")
	withdrawn := input(t, "withdrawn")
	require.NoError(t, repo.ApplyEvent(ctx, submitted))
	require.NoError(t, repo.ApplyEvent(ctx, submitted))
	counts(t, ctx, out, 1)
	e, err := out.Pending(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, "application-status", e[0].Producer)
	var changed eventsv1.StatusChanged
	require.NoError(t, e[0].Data.UnmarshalTo(&changed))
	require.Equal(t, "UNSPECIFIED", changed.PreviousStatus)
	require.Equal(t, "SUBMITTED", changed.NewStatus)
	require.Equal(t, submitted.ApplicantID, changed.ApplicantId)
	require.NotEqual(t, submitted.EventID, e[0].EventId)
	// A distinct submission ID is a conflict, not an expected duplicate.
	conflict := submitted
	conflict.EventID = uuid.NewString()
	require.ErrorIs(t, repo.ApplyEvent(ctx, conflict), domain.ErrConflictingEvent)
	wrong := withdrawn
	wrong.ApplicantID = uuid.NewString()
	require.ErrorIs(t, repo.ApplyEvent(ctx, wrong), domain.ErrConflictingEvent)
	counts(t, ctx, out, 1)
	// Same input ID raced by several workers commits once.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.ApplyEvent(ctx, withdrawn) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	counts(t, ctx, out, 2)
	s, err := repo.Get(ctx, submitted.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusWithdrawn, s.Status)
	history, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, history, 2)
	// Every final state forbids a fresh withdrawal.
	for _, final := range []domain.Status{domain.StatusAccepted, domain.StatusDeclined, domain.StatusRejected, domain.StatusWithdrawn} {
		_, err = pool.Exec(ctx, "UPDATE application_statuses SET status=$2 WHERE application_id=$1", s.ApplicationID, final.String())
		require.NoError(t, err)
		bad := withdrawn
		bad.EventID = uuid.NewString()
		require.ErrorIs(t, repo.ApplyEvent(ctx, bad), domain.ErrInvalidTransition)
	}
	var processed int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events").Scan(&processed))
	require.Equal(t, 2, processed)
	// Force a late outbox error: none of status/history/input ID may remain.
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events ADD CONSTRAINT force_failure CHECK (false) NOT VALID")
	require.NoError(t, err)
	fresh := submitted
	fresh.ApplicationID = uuid.NewString()
	fresh.EventID = uuid.NewString()
	require.Error(t, repo.ApplyEvent(ctx, fresh))
	_, err = repo.Get(ctx, fresh.ApplicationID)
	require.ErrorIs(t, err, domain.ErrApplicationNotFound)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events").Scan(&processed))
	require.Equal(t, 2, processed)
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events DROP CONSTRAINT force_failure")
	require.NoError(t, err)
	require.NoError(t, repo.ApplyEvent(ctx, fresh))
	counts(t, ctx, out, 3)
	// History insertion failure also rolls back the status and input claim.
	_, err = pool.Exec(ctx, "ALTER TABLE status_history ADD CONSTRAINT force_history_failure CHECK (false) NOT VALID")
	require.NoError(t, err)
	w := withdrawn
	w.ApplicationID = fresh.ApplicationID
	w.EventID = uuid.NewString()
	require.Error(t, repo.ApplyEvent(ctx, w))
	s, err = repo.Get(ctx, fresh.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusSubmitted, s.Status)
	_, err = pool.Exec(ctx, "ALTER TABLE status_history DROP CONSTRAINT force_history_failure")
	require.NoError(t, err)
	require.NoError(t, repo.ApplyEvent(ctx, w))
	counts(t, ctx, out, 4)
	// Distinct IDs racing to withdraw: one change, one final-state conflict.
	race := submitted
	race.ApplicationID = uuid.NewString()
	race.EventID = uuid.NewString()
	require.NoError(t, repo.ApplyEvent(ctx, race))
	errs = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := withdrawn
			v.EventID = uuid.NewString()
			v.ApplicationID = race.ApplicationID
			errs <- repo.ApplyEvent(ctx, v)
		}()
	}
	wg.Wait()
	close(errs)
	success, failed := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, domain.ErrInvalidTransition)
			failed++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, failed)
	counts(t, ctx, out, 6)
	// Missing input can be retried when submission arrives.
	unknown := withdrawn
	unknown.ApplicationID = uuid.NewString()
	unknown.EventID = uuid.NewString()
	require.ErrorIs(t, repo.ApplyEvent(ctx, unknown), domain.ErrApplicationNotFound)
	require.Error(t, func() error { _, err := out.Pending(ctx, 0); return err }())
}

// failOnce proves a failed publish leaves persisted IDs available for retry.
type failOnce struct {
	next   *rabbitmq.Publisher
	failed bool
}

func (p *failOnce) Publish(ctx context.Context, e *eventsv1.EventEnvelope) error {
	if !p.failed {
		p.failed = true
		return errors.New("synthetic transport failure")
	}
	return p.next.Publish(ctx, e)
}

func TestApplicationEvents_BrokerConsumerAndConfirmedRelay(t *testing.T) {
	ctx, pool := startStatusDB(t)
	url := startRelayBroker(t, ctx)
	repo, out := postgres.NewStatuses(pool), postgres.NewOutbox(pool)
	pub, err := rabbitmq.NewPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	conn, err := amqp.Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "application.status_changed", "jobtrack.events", false, nil))
	received, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)
	workerCtx, cancel := context.WithCancel(ctx)
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	cfg := rabbitmq.ApplicationConsumerConfig(url, slog.New(slog.NewTextHandler(io.Discard, nil)), repo)
	cfg.OnReady = func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	go func() { done <- events.Consume(workerCtx, cfg) }()
	defer func() { cancel(); require.NoError(t, <-done) }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("consumer never became ready")
	}
	submitted, withdrawn := eventFixture(t, "submitted"), eventFixture(t, "withdrawn")
	require.NoError(t, pub.Publish(ctx, submitted))
	require.NoError(t, pub.Publish(ctx, submitted))
	require.Eventually(t, func() bool {
		v, err := repo.Get(ctx, input(t, "submitted").ApplicationID)
		return err == nil && v.Status == domain.StatusSubmitted
	}, 10*time.Second, 20*time.Millisecond)
	require.NoError(t, pub.Publish(ctx, withdrawn))
	require.NoError(t, pub.Publish(ctx, withdrawn))
	require.Eventually(t, func() bool {
		v, err := repo.Get(ctx, input(t, "submitted").ApplicationID)
		return err == nil && v.Status == domain.StatusWithdrawn
	}, 10*time.Second, 20*time.Millisecond)
	counts(t, ctx, out, 2)
	pending, err := out.Pending(ctx, 100)
	require.NoError(t, err)
	relay := app.NewOutboxRelay(out, &failOnce{next: pub}, time.Now)
	n, err := relay.PublishPending(ctx)
	require.Error(t, err)
	require.Zero(t, n)
	counts(t, ctx, out, 2)
	retry, err := out.Pending(ctx, 100)
	require.NoError(t, err)
	require.True(t, proto.Equal(pending[0], retry[0]))
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	counts(t, ctx, out, 0)
	for _, want := range pending {
		select {
		case delivery := <-received:
			got, err := events.UnmarshalEnvelope(delivery.Body)
			require.NoError(t, err)
			require.True(t, proto.Equal(want, got))
		case <-ctx.Done():
			t.Fatal("no confirmed event received")
		}
	}
	// Invalid producer reaches this service's durable dead queue.
	malformed := proto.Clone(submitted).(*eventsv1.EventEnvelope)
	malformed.EventId = uuid.NewString()
	malformed.Producer = "offer"
	require.NoError(t, pub.Publish(ctx, malformed))
	require.Eventually(t, func() bool {
		msg, ok, err := ch.Get(rabbitmq.ApplicationQueue+".dead", true)
		if err != nil || !ok {
			return false
		}
		got, err := events.UnmarshalEnvelope(msg.Body)
		return err == nil && got.EventId == malformed.EventId
	}, 10*time.Second, 50*time.Millisecond)
	require.NoError(t, out.MarkPublished(ctx, pending[0].EventId, time.Now()))
}
