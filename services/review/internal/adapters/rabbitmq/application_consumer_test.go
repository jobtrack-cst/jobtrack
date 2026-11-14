package rabbitmq_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

type recorder struct {
	err     error
	eventID string
	got     []domain.ApplicationParticipants
}

func (r *recorder) RecordApplication(_ context.Context, eventID string, p domain.ApplicationParticipants) error {
	r.eventID = eventID
	r.got = append(r.got, p)
	return r.err
}

func fixture(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	return e
}

func withData(t *testing.T, e *eventsv1.EventEnvelope, change func(*eventsv1.ApplicationSubmitted)) *eventsv1.EventEnvelope {
	t.Helper()
	var data eventsv1.ApplicationSubmitted
	require.NoError(t, e.GetData().UnmarshalTo(&data))
	change(&data)
	packed, err := anypb.New(&data)
	require.NoError(t, err)
	c := proto.Clone(e).(*eventsv1.EventEnvelope)
	c.Data = packed
	return c
}

func TestApplicationConsumerConfig(t *testing.T) {
	cfg := rabbitmq.ApplicationConsumerConfig("amqp://example", slog.New(slog.NewTextHandler(io.Discard, nil)), &recorder{})
	assert.Equal(t, "review.applications", cfg.Queue)
	assert.Equal(t, []string{"application.submitted"}, cfg.Bindings)
	assert.Equal(t, "amqp://example", cfg.URL)
	assert.NotNil(t, cfg.Handler)
}

func TestApplicationHandler_RecordsSharedFixture(t *testing.T) {
	e := fixture(t)
	r := &recorder{}
	require.NoError(t, rabbitmq.ApplicationHandler(r)(context.Background(), e))
	require.Len(t, r.got, 1)
	assert.Equal(t, e.GetEventId(), r.eventID)
	assert.Equal(t, domain.ApplicationParticipants{
		ApplicationID: "11111111-1111-4111-8111-111111111111",
		ApplicantID:   "22222222-2222-4222-8222-222222222222",
		EmployerID:    "55555555-5555-4555-8555-555555555555",
	}, r.got[0])
}

func TestApplicationHandler_NormalizesUppercaseIDs(t *testing.T) {
	e := withData(t, fixture(t), func(d *eventsv1.ApplicationSubmitted) {
		d.ApplicationId = strings.ToUpper(d.ApplicationId)
		d.EmployerId = strings.ToUpper(d.EmployerId)
	})
	r := &recorder{}
	require.NoError(t, rabbitmq.ApplicationHandler(r)(context.Background(), e))
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", r.got[0].ApplicationID)
	assert.Equal(t, "55555555-5555-4555-8555-555555555555", r.got[0].EmployerID)
}

func TestApplicationHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope
	}{
		{"wrong producer", func(_ *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope { e.Producer = "offer"; return e }},
		{"wrong type", func(_ *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			e.Type = "application.withdrawn"
			return e
		}},
		{"event_id not a UUID", func(_ *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope { e.EventId = "bad"; return e }},
		{"missing occurred_at", func(_ *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope { e.OccurredAt = nil; return e }},
		{"missing data", func(_ *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope { e.Data = nil; return e }},
		{"data of another event", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			packed, err := anypb.New(&eventsv1.ApplicationWithdrawn{ApplicationId: "11111111-1111-4111-8111-111111111111"})
			require.NoError(t, err)
			e.Data = packed
			return e
		}},
		{"application_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			return withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ApplicationId = "x" })
		}},
		{"applicant_id missing", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			return withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ApplicantId = "" })
		}},
		{"employer_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			return withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.EmployerId = "employer" })
		}},
		{"job_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			return withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.JobId = "1" })
		}},
		{"resume_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) *eventsv1.EventEnvelope {
			return withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ResumeId = "1" })
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &recorder{}
			e := tt.change(t, proto.Clone(fixture(t)).(*eventsv1.EventEnvelope))
			err := rabbitmq.ApplicationHandler(r)(context.Background(), e)
			require.ErrorIs(t, err, events.ErrPermanent)
			assert.Empty(t, r.got, "nothing recorded")
		})
	}
	t.Run("nil envelope", func(t *testing.T) {
		require.ErrorIs(t, rabbitmq.ApplicationHandler(&recorder{})(context.Background(), nil), events.ErrPermanent)
	})
}

func TestApplicationHandler_RecorderErrors(t *testing.T) {
	t.Run("conflicting participants are dead-lettered", func(t *testing.T) {
		err := rabbitmq.ApplicationHandler(&recorder{err: domain.ErrConflictingParticipants})(context.Background(), fixture(t))
		require.ErrorIs(t, err, events.ErrPermanent)
		require.ErrorIs(t, err, domain.ErrConflictingParticipants)
	})
	t.Run("database errors are retried", func(t *testing.T) {
		dbErr := errors.New("record application failed")
		err := rabbitmq.ApplicationHandler(&recorder{err: dbErr})(context.Background(), fixture(t))
		require.ErrorIs(t, err, dbErr)
		assert.NotErrorIs(t, err, events.ErrPermanent)
	})
}
