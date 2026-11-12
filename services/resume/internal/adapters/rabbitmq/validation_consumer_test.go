package rabbitmq

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type validationAck struct {
	ack, retry, dead int
	err              error
}

func (a *validationAck) Ack(_ uint64, multiple bool) error {
	if multiple {
		panic("multiple ack")
	}
	a.ack++
	return a.err
}
func (a *validationAck) Nack(_ uint64, multiple, requeue bool) error {
	if multiple || requeue {
		panic("invalid nack")
	}
	a.dead++
	return a.err
}
func (a *validationAck) Reject(_ uint64, requeue bool) error {
	if !requeue {
		panic("invalid reject")
	}
	a.retry++
	return a.err
}
func validationFixture(t *testing.T, name string) amqp.Delivery {
	t.Helper()
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/resume." + name + ".json")
	require.NoError(t, err)
	return amqp.Delivery{Body: body, Exchange: "jobtrack.events", RoutingKey: "resume." + name, DeliveryTag: 1}
}

func TestValidationHandler_SettlesOnlyAfterProcessing(t *testing.T) {
	for _, tc := range []struct {
		name             string
		outcome          domain.ValidationOutcome
		err              error
		ack, retry, dead int
	}{
		{"applied", domain.ValidationApplied, nil, 1, 0, 0}, {"duplicate", domain.ValidationDuplicate, nil, 1, 0, 0}, {"final", domain.ValidationAlreadyFinal, nil, 1, 0, 0},
		{"unknown", 0, domain.ErrResumeNotFound, 0, 1, 0}, {"mismatch", 0, domain.ErrValidationApplicantMismatch, 0, 0, 1},
		{"invalid", 0, domain.ErrInvalidValidationResult, 0, 0, 1}, {"database", 0, errors.New("private driver details"), 0, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockValidationRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			clock.EXPECT().Now().Return(time.Now())
			a := &validationAck{}
			d := validationFixture(t, "validated")
			d.Acknowledger = a
			repo.EXPECT().ApplyValidation(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, r domain.ValidationResult, at time.Time) (domain.ValidationOutcome, error) {
				require.Zero(t, a.ack)
				require.Zero(t, a.dead)
				require.Zero(t, a.retry)
				require.Equal(t, "00000000-0000-4000-8000-000000000002", r.EventID)
				return tc.outcome, tc.err
			})
			h := ValidationHandler{Results: app.NewValidationResults(repo, clock)}
			require.NoError(t, h.Handle(context.Background(), d))
			require.Equal(t, tc.ack, a.ack)
			require.Equal(t, tc.retry, a.retry)
			require.Equal(t, tc.dead, a.dead)
		})
	}
}

func TestValidationHandler_InvalidDeliveryAndBudget(t *testing.T) {
	for _, scenario := range []string{"malformed", "routing", "producer", "payload", "type", "missing count on redelivery", "bad count", "negative count", "budget exceeded", "reason empty", "reason 501"} {
		t.Run(scenario, func(t *testing.T) {
			d := validationFixture(t, "validated")
			a := &validationAck{}
			d.Acknowledger = a
			switch scenario {
			case "malformed":
				d.Body = []byte("bad")
			case "routing":
				d.RoutingKey = "resume.rejected"
			case "producer":
				d.Body = []byte(strings.ReplaceAll(string(d.Body), "resume-validator", "auth"))
			case "payload":
				d.Body = []byte(strings.ReplaceAll(string(d.Body), "ResumeValidated", "ResumeUploaded"))
			case "type":
				d.Body = []byte(strings.ReplaceAll(string(d.Body), "resume.validated", "resume.uploaded"))
				d.RoutingKey = "resume.uploaded"
			case "missing count on redelivery":
				d.Redelivered = true
			case "bad count":
				d.Headers = amqp.Table{"x-delivery-count": "0"}
			case "negative count":
				d.Headers = amqp.Table{"x-delivery-count": int64(-1)}
			case "budget exceeded":
				d.Headers = amqp.Table{"x-delivery-count": int64(4)}
			case "reason empty", "reason 501":
				d = validationFixture(t, "rejected")
				d.Acknowledger = a
				reason := ""
				if scenario == "reason 501" {
					reason = strings.Repeat("a", 501)
				}
				d.Body = []byte(strings.ReplaceAll(string(d.Body), "Invalid PDF signature", reason))
			}
			h := ValidationHandler{} // Invalid deliveries must never invoke application code.
			require.NoError(t, h.Handle(context.Background(), d))
			require.Equal(t, 1, a.dead)
			require.Zero(t, a.ack)
			require.Zero(t, a.retry)
		})
	}
}

func TestValidationHandler_FourDeliveriesAndSettlementUncertainty(t *testing.T) {
	for _, count := range []int64{0, 1, 2, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockValidationRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			clock.EXPECT().Now().Return(time.Now())
			repo.EXPECT().ApplyValidation(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.ValidationOutcomeUnspecified, domain.ErrResumeNotFound)
			d := validationFixture(t, "validated")
			a := &validationAck{}
			d.Acknowledger = a
			d.Headers = amqp.Table{"x-delivery-count": count}
			h := ValidationHandler{Results: app.NewValidationResults(repo, clock)}
			require.NoError(t, h.Handle(context.Background(), d))
			if count == 3 {
				require.Equal(t, 1, a.dead)
				require.Zero(t, a.retry)
			} else {
				require.Equal(t, 1, a.retry)
				require.Zero(t, a.dead)
			}
			require.Zero(t, a.ack)
		})
	}
	d := validationFixture(t, "validated")
	a := &validationAck{err: errors.New("settlement uncertain")}
	d.Acknowledger = a
	d.Body = []byte("bad")
	require.Error(t, (&ValidationHandler{}).Handle(context.Background(), d))
	require.Equal(t, 1, a.dead)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a = &validationAck{}
	d.Acknowledger = a
	require.ErrorIs(t, (&ValidationHandler{}).Handle(ctx, d), context.Canceled)
	require.Zero(t, a.ack+a.retry+a.dead)
	ctx, cancel = context.WithCancel(context.Background())
	d = validationFixture(t, "validated")
	d.Headers = amqp.Table{"x-delivery-count": int64(3)}
	d.Acknowledger = a
	cancel()
	require.ErrorIs(t, (&ValidationHandler{}).Handle(ctx, d), context.Canceled)
}

type validationTopologySpy struct {
	t     *testing.T
	fail  string
	calls []string
}

func (s *validationTopologySpy) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	s.calls = append(s.calls, "exchange")
	require.Equal(s.t, "jobtrack.events", name)
	require.Equal(s.t, "topic", kind)
	require.True(s.t, durable)
	require.False(s.t, autoDelete || internal || noWait)
	if s.fail == "exchange" {
		return errors.New("private-detail")
	}
	return nil
}
func (s *validationTopologySpy) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	s.calls = append(s.calls, name)
	require.True(s.t, durable)
	require.False(s.t, autoDelete || exclusive || noWait)
	want := amqp.Table{"x-queue-type": "quorum"}
	if name == ValidationQueue {
		want = amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int32(3), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": ValidationDeadQueue, "x-dead-letter-strategy": "at-least-once", "x-overflow": "reject-publish"}
	} else {
		require.Equal(s.t, ValidationDeadQueue, name)
	}
	require.Equal(s.t, want, args)
	if s.fail == name {
		return amqp.Queue{}, errors.New("private-detail")
	}
	return amqp.Queue{Name: name}, nil
}
func (s *validationTopologySpy) QueueBind(queue, key, exchange string, noWait bool, args amqp.Table) error {
	s.calls = append(s.calls, key)
	require.Equal(s.t, ValidationQueue, queue)
	require.Equal(s.t, "jobtrack.events", exchange)
	require.False(s.t, noWait)
	if s.fail == key {
		return errors.New("private-detail")
	}
	return nil
}
func TestValidationTopology_ExactBindingsAndFailClosed(t *testing.T) {
	expected := []string{"exchange", ValidationDeadQueue, ValidationQueue, "resume.validated", "resume.rejected"}
	for i, stage := range append([]string{""}, expected...) {
		t.Run(stage, func(t *testing.T) {
			s := &validationTopologySpy{t: t, fail: stage}
			err := DeclareValidationTopology(s)
			want := expected
			if stage != "" {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-detail")
				want = expected[:i]
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, want, s.calls)
		})
	}
}

func TestValidationHandler_AckAndRejectFailuresEndSession(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(map[bool]string{false: "reject", true: "ack"}[success], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockValidationRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			clock.EXPECT().Now().Return(time.Now())
			outcome := domain.ValidationOutcomeUnspecified
			var err error = domain.ErrResumeNotFound
			if success {
				outcome = domain.ValidationApplied
				err = nil
			}
			repo.EXPECT().ApplyValidation(gomock.Any(), gomock.Any(), gomock.Any()).Return(outcome, err)
			ack := &validationAck{err: errors.New("uncertain settlement")}
			d := validationFixture(t, "validated")
			d.Acknowledger = ack
			h := &ValidationHandler{Results: app.NewValidationResults(repo, clock)}
			require.Error(t, h.Handle(context.Background(), d))
			require.Equal(t, 1, ack.ack+ack.retry+ack.dead)
		})
	}
}
