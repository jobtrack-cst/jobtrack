package rabbitmq_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type applier struct {
	calls int
	err   error
	input domain.ApplicationEvent
}

func (a *applier) ApplyEvent(_ context.Context, e domain.ApplicationEvent) error {
	a.calls++
	a.input = e
	return a.err
}

func fixture(t *testing.T, kind string) *eventsv1.EventEnvelope {
	t.Helper()
	b, err := os.ReadFile("../../../../../tests/contract/fixtures/application." + kind + ".json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	return e
}

func TestApplicationHandler_Fixtures(t *testing.T) {
	for _, kind := range []string{"submitted", "withdrawn"} {
		t.Run(kind, func(t *testing.T) {
			a := new(applier)
			e := fixture(t, kind)
			require.NoError(t, rabbitmq.ApplicationHandler(a)(context.Background(), e))
			require.Equal(t, 1, a.calls)
			require.Equal(t, e.EventId, a.input.EventID)
			require.Equal(t, kind == "submitted", a.input.Submitted)
			cfg := rabbitmq.ApplicationConsumerConfig("amqp://test", slog.New(slog.NewTextHandler(io.Discard, nil)), a)
			require.Equal(t, "application-status.applications", cfg.Queue)
			require.Equal(t, []string{"application.submitted", "application.withdrawn"}, cfg.Bindings)
		})
	}
}

func TestApplicationHandler_RejectsMalformed(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*eventsv1.EventEnvelope)
	}{
		{"producer", func(e *eventsv1.EventEnvelope) { e.Producer = "offer" }},
		{"routing type", func(e *eventsv1.EventEnvelope) { e.Type = "offer.made" }},
		{"event ID", func(e *eventsv1.EventEnvelope) { e.EventId = "bad" }},
		{"missing timestamp", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"invalid timestamp", func(e *eventsv1.EventEnvelope) { e.OccurredAt = &timestamppb.Timestamp{Seconds: 253402300800} }},
		{"missing payload", func(e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"wrong Any", func(e *eventsv1.EventEnvelope) { e.Data, _ = anypb.New(&eventsv1.ApplicationWithdrawn{}) }},
		{"invalid participant", func(e *eventsv1.EventEnvelope) {
			d := new(eventsv1.ApplicationSubmitted)
			_ = e.Data.UnmarshalTo(d)
			d.ApplicantId = "bad"
			e.Data, _ = anypb.New(d)
		}},
		{"invalid job", func(e *eventsv1.EventEnvelope) {
			d := new(eventsv1.ApplicationSubmitted)
			_ = e.Data.UnmarshalTo(d)
			d.JobId = "bad"
			e.Data, _ = anypb.New(d)
		}},
		{"invalid resume", func(e *eventsv1.EventEnvelope) {
			d := new(eventsv1.ApplicationSubmitted)
			_ = e.Data.UnmarshalTo(d)
			d.ResumeId = "bad"
			e.Data, _ = anypb.New(d)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := proto.Clone(fixture(t, "submitted")).(*eventsv1.EventEnvelope)
			tt.mutate(e)
			a := new(applier)
			require.ErrorIs(t, rabbitmq.ApplicationHandler(a)(context.Background(), e), events.ErrPermanent)
			require.Zero(t, a.calls)
		})
	}
	require.ErrorIs(t, rabbitmq.ApplicationHandler(new(applier))(context.Background(), nil), events.ErrPermanent)
}

func TestApplicationHandler_ClassifiesFailures(t *testing.T) {
	for _, err := range []error{domain.ErrConflictingEvent, domain.ErrInvalidTransition, domain.ErrApplicationNotFound, errors.New("database unavailable")} {
		a := &applier{err: err}
		got := rabbitmq.ApplicationHandler(a)(context.Background(), fixture(t, "withdrawn"))
		require.ErrorIs(t, got, err)
		require.Equal(t, errors.Is(err, domain.ErrConflictingEvent) || errors.Is(err, domain.ErrInvalidTransition), errors.Is(got, events.ErrPermanent))
	}
}
