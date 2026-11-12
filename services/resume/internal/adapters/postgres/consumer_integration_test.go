//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	rabbit "github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type validationRepoFunc func(context.Context, domain.ValidationResult, time.Time) (domain.ValidationOutcome, error)

func (f validationRepoFunc) ApplyValidation(ctx context.Context, r domain.ValidationResult, at time.Time) (domain.ValidationOutcome, error) {
	return f(ctx, r, at)
}

func TestValidationConsumer_Integration(t *testing.T) {
	ctx, pool := startResumeDB(t)
	broker := dispatcherBroker(t, ctx)
	require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.Confirm(false))
	repo := postgres.NewValidation(pool)
	resumes := postgres.NewResumes(pool)
	seed := func() domain.ResumeFile {
		t.Helper()
		f := newFile(applicantA, base, pdf(100))
		_, err := resumes.Create(ctx, f, uploaded(t, f))
		require.NoError(t, err)
		return f
	}
	body := func(f domain.ResumeFile, rejected bool) ([]byte, string) {
		t.Helper()
		var payload proto.Message = &eventsv1.ResumeValidated{ResumeId: f.ID, ApplicantId: f.ApplicantID}
		kind := "resume.validated"
		if rejected {
			kind = "resume.rejected"
			payload = &eventsv1.ResumeRejected{ResumeId: f.ID, ApplicantId: f.ApplicantID, Reason: "  rejected reason  "}
		}
		data, err := anypb.New(payload)
		require.NoError(t, err)
		b, err := events.MarshalEnvelope(&eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: kind, Producer: "resume-validator", OccurredAt: timestamppb.New(base), Data: data})
		require.NoError(t, err)
		return b, kind
	}
	publish := func(b []byte, key string) {
		t.Helper()
		confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "jobtrack.events", key, true, false, amqp.Publishing{Body: b, DeliveryMode: amqp.Persistent, ContentType: "application/json"})
		require.NoError(t, err)
		ok, err := confirm.WaitContext(ctx)
		require.NoError(t, err)
		require.True(t, ok)
	}
	start := func(t *testing.T, store ports.ValidationRepository) func() {
		t.Helper()
		cctx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		consumer := &rabbit.ValidationConsumer{Broker: broker, Handler: &rabbit.ValidationHandler{Results: app.NewValidationResults(store, outboxClock{})}}
		go func() { done <- consumer.Session(cctx) }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("consumer session did not stop")
				}
			})
		}
		t.Cleanup(stop)
		require.Eventually(t, func() bool {
			q, err := ch.QueueDeclarePassive(rabbit.ValidationQueue, true, false, false, false, nil)
			return err == nil && q.Consumers >= 1
		}, 3*time.Second, 10*time.Millisecond)
		return stop
	}
	processed := func(id string) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id=$1`, id).Scan(&n))
		return n
	}
	dead := func(t *testing.T, original []byte) {
		t.Helper()
		var msg amqp.Delivery
		require.Eventually(t, func() bool {
			m, ok, err := ch.Get(rabbit.ValidationDeadQueue, false)
			if err == nil && ok {
				msg = m
				return true
			}
			return false
		}, 15*time.Second, 20*time.Millisecond)
		require.Equal(t, original, msg.Body)
		require.NoError(t, msg.Ack(false))
	}
	t.Run("valid rejected duplicate and final", func(t *testing.T) {
		stop := start(t, repo)
		defer stop()
		for _, rejected := range []bool{false, true} {
			f := seed()
			b, key := body(f, rejected)
			e, err := events.UnmarshalEnvelope(b)
			require.NoError(t, err)
			publish(b, key)
			publish(b, key)
			require.Eventually(t, func() bool { return processed(e.EventId) == 1 }, 3*time.Second, 10*time.Millisecond)
			other, otherKey := body(f, !rejected)
			publish(other, otherKey)
			otherE, err := events.UnmarshalEnvelope(other)
			require.NoError(t, err)
			require.Eventually(t, func() bool { return processed(otherE.EventId) == 1 }, 3*time.Second, 10*time.Millisecond)
			got, err := resumes.GetByID(ctx, f.ID)
			require.NoError(t, err)
			expected := domain.ValidationStatusValidated
			reason := ""
			if rejected {
				expected = domain.ValidationStatusRejected
				reason = "rejected reason"
			}
			require.Equal(t, expected, got.ValidationStatus)
			require.Equal(t, reason, got.RejectionReason)
		}
	})
	t.Run("two consumers preserve first final under conflicting events", func(t *testing.T) {
		stop1 := start(t, repo)
		defer stop1()
		stop2 := start(t, repo)
		defer stop2()
		require.Eventually(t, func() bool {
			q, err := ch.QueueDeclarePassive(rabbit.ValidationQueue, true, false, false, false, nil)
			return err == nil && q.Consumers == 2
		}, 3*time.Second, 10*time.Millisecond)
		f := seed()
		b1, k1 := body(f, false)
		b2, k2 := body(f, true)
		e1, err := events.UnmarshalEnvelope(b1)
		require.NoError(t, err)
		e2, err := events.UnmarshalEnvelope(b2)
		require.NoError(t, err)
		publish(b1, k1)
		publish(b2, k2)
		require.Eventually(t, func() bool { return processed(e1.EventId) == 1 && processed(e2.EventId) == 1 }, 3*time.Second, 10*time.Millisecond)
		got, err := resumes.GetByID(ctx, f.ID)
		require.NoError(t, err)
		require.Contains(t, []domain.ValidationStatus{domain.ValidationStatusValidated, domain.ValidationStatusRejected}, got.ValidationStatus)
		original := got
		publish(b1, k1)
		publish(b2, k2)
		require.Eventually(t, func() bool {
			q, err := ch.QueueDeclarePassive(rabbit.ValidationQueue, true, false, false, false, nil)
			return err == nil && q.Messages == 0
		}, 3*time.Second, 10*time.Millisecond)
		after, err := resumes.GetByID(ctx, f.ID)
		require.NoError(t, err)
		require.Equal(t, original, after)
	})

	t.Run("unknown bounded retry", func(t *testing.T) {
		f := newFile(applicantA, base, pdf(100))
		b, key := body(f, false)
		e, err := events.UnmarshalEnvelope(b)
		require.NoError(t, err)
		var calls atomic.Int32
		store := validationRepoFunc(func(c context.Context, r domain.ValidationResult, at time.Time) (domain.ValidationOutcome, error) {
			// Earlier sessions may replay already-committed events after an uncertain ack.
			// Count this event's delivery budget, not unrelated idempotent replays.
			if r.EventID == e.EventId {
				calls.Add(1)
			}
			return repo.ApplyValidation(c, r, at)
		})
		stop := start(t, store)
		defer stop()
		publish(b, key)
		dead(t, b)
		require.Equal(t, int32(4), calls.Load())
		require.Zero(t, processed(e.EventId))
	})

	t.Run("mismatch and malformed immediate DLQ", func(t *testing.T) {
		stop := start(t, repo)
		defer stop()
		f := seed()
		wrong := f
		wrong.ApplicantID = applicantB
		b, key := body(wrong, false)
		publish(b, key)
		dead(t, b)
		e, err := events.UnmarshalEnvelope(b)
		require.NoError(t, err)
		require.Zero(t, processed(e.EventId))
		got, err := resumes.GetByID(ctx, f.ID)
		require.NoError(t, err)
		require.Equal(t, domain.ValidationStatusPending, got.ValidationStatus)
		publish([]byte("malformed retained payload"), "resume.validated")
		dead(t, []byte("malformed retained payload"))
	})
	t.Run("transient database failure retains event ID", func(t *testing.T) {
		f := seed()
		b, key := body(f, false)
		e, err := events.UnmarshalEnvelope(b)
		require.NoError(t, err)
		var calls atomic.Int32
		store := validationRepoFunc(func(c context.Context, r domain.ValidationResult, at time.Time) (domain.ValidationOutcome, error) {
			if r.EventID == e.EventId && calls.Add(1) == 1 {
				return 0, errors.New("synthetic transient DB error")
			}
			return repo.ApplyValidation(c, r, at)
		})
		stop := start(t, store)
		defer stop()
		publish(b, key)
		require.Eventually(t, func() bool { return processed(e.EventId) == 1 }, 5*time.Second, 10*time.Millisecond)
		require.Equal(t, int32(2), calls.Load())
	})
	t.Run("four interrupted deliveries survive session restarts", func(t *testing.T) {
		f := newFile(applicantA, base, pdf(100))
		b, key := body(f, false)
		e, err := events.UnmarshalEnvelope(b)
		require.NoError(t, err)
		calls := make(chan string, 8)
		store := validationRepoFunc(func(c context.Context, r domain.ValidationResult, at time.Time) (domain.ValidationOutcome, error) {
			if r.EventID != e.EventId {
				return repo.ApplyValidation(c, r, at)
			}
			calls <- r.EventID
			<-c.Done()
			return 0, c.Err()
		})
		for i := 0; i < 4; i++ {
			stop := start(t, store)
			if i == 0 {
				publish(b, key)
			}
			select {
			case id := <-calls:
				require.Equal(t, e.EventId, id)
			case <-time.After(6 * time.Second):
				t.Fatal("delivery missing after restart")
			}
			stop()
		}
		dead(t, b)
		require.Zero(t, processed(e.EventId))
		require.Empty(t, calls)
	})
	t.Run("fixtures drive actual consumer decoder", func(t *testing.T) {
		for _, name := range []string{"validated", "rejected"} {
			b, err := os.ReadFile("../../../../../tests/contract/fixtures/resume." + name + ".json")
			require.NoError(t, err)
			r, err := rabbit.DecodeValidationDelivery(amqp.Delivery{Body: b, Exchange: "jobtrack.events", RoutingKey: "resume." + name})
			require.NoError(t, err)
			require.NotEmpty(t, r.EventID)
		}
	})
}

func TestValidationConsumer_IntegrationTopologyCompatibility(t *testing.T) {
	for _, queue := range []string{rabbit.ValidationQueue, rabbit.ValidationDeadQueue} {
		t.Run(queue, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			broker := dispatcherBroker(t, ctx)
			conn, err := amqp.Dial(broker)
			require.NoError(t, err)
			defer conn.Close()
			ch, err := conn.Channel()
			require.NoError(t, err)
			defer ch.Close()
			_, err = ch.QueueDeclare(queue, true, false, false, false, amqp.Table{"x-queue-type": "classic"})
			require.NoError(t, err)
			require.NoError(t, ch.Confirm(false))
			original := []byte("synthetic preserved validation event")
			confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{Body: original, DeliveryMode: amqp.Persistent})
			require.NoError(t, err)
			ok, err := confirm.WaitContext(ctx)
			require.NoError(t, err)
			require.True(t, ok)
			err = rabbit.ProvisionTopology(ctx, broker)
			require.ErrorContains(t, err, "reviewed migration")
			msg, ok, err := ch.Get(queue, false)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, original, msg.Body)
			require.NoError(t, msg.Ack(false))
		})
	}
	t.Run("exact binding and idempotent initialization", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		broker := dispatcherBroker(t, ctx)
		require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
		require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
		conn, err := amqp.Dial(broker)
		require.NoError(t, err)
		defer conn.Close()
		ch, err := conn.Channel()
		require.NoError(t, err)
		defer ch.Close()
		require.NoError(t, ch.Confirm(false))
		for _, key := range []string{"resume.validated", "resume.rejected", "resume.uploaded"} {
			confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "jobtrack.events", key, true, false, amqp.Publishing{Body: []byte(key), DeliveryMode: amqp.Persistent})
			require.NoError(t, err)
			ok, err := confirm.WaitContext(ctx)
			require.NoError(t, err)
			require.True(t, ok)
		}
		require.NoError(t, rabbit.ProvisionTopology(ctx, broker))
		q, err := ch.QueueDeclarePassive(rabbit.ValidationQueue, true, false, false, false, nil)
		require.NoError(t, err)
		require.Equal(t, 2, q.Messages)
		require.Zero(t, q.Consumers)
		for _, key := range []string{"resume.validated", "resume.rejected"} {
			msg, ok, err := ch.Get(q.Name, false)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, []byte(key), msg.Body)
			require.NoError(t, msg.Ack(false))
		}
	})
}
