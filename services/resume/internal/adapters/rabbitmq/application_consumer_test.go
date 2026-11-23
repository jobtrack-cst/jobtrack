package rabbitmq_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
)

const fixtureEventID = "66666666-6666-4666-8666-666666666665"

// fixtureEnvelope is the shared contract fixture (tests/contract/fixtures).
func fixtureEnvelope(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	return e
}

func fixtureRow() domain.ApplicationResume {
	return domain.ApplicationResume{
		ApplicationID: "11111111-1111-4111-8111-111111111111", ApplicantID: "22222222-2222-4222-8222-222222222222",
		EmployerID: "55555555-5555-4555-8555-555555555555", ResumeID: "44444444-4444-4444-8444-444444444444",
	}
}

func withData(t *testing.T, e *eventsv1.EventEnvelope, edit func(*eventsv1.ApplicationSubmitted)) {
	t.Helper()
	var data eventsv1.ApplicationSubmitted
	require.NoError(t, e.Data.UnmarshalTo(&data))
	edit(&data)
	packed, err := anypb.New(&data)
	require.NoError(t, err)
	e.Data = packed
}

func TestApplicationConsumerConfig_UsesTheJT114Queue(t *testing.T) {
	repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
	cfg := rabbitmq.ApplicationConsumerConfig("amqp://synthetic", logging.New("test", io.Discard), repo)
	require.Equal(t, "resume.applications", cfg.Queue)
	require.Equal(t, []string{"application.submitted"}, cfg.Bindings)
	require.NotNil(t, cfg.Handler)
}

func TestApplicationHandler_StoresValidEvents(t *testing.T) {
	t.Run("shared fixture", func(t *testing.T) {
		repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
		repo.EXPECT().Record(gomock.Any(), fixtureEventID, fixtureRow()).Return(nil)
		require.NoError(t, rabbitmq.ApplicationHandler(repo)(context.Background(), fixtureEnvelope(t)))
	})
	t.Run("uppercase IDs are stored lowercase", func(t *testing.T) {
		e := fixtureEnvelope(t)
		e.EventId = strings.ToUpper(e.EventId)
		withData(t, e, func(d *eventsv1.ApplicationSubmitted) {
			d.ApplicationId, d.ApplicantId = strings.ToUpper(d.ApplicationId), strings.ToUpper(d.ApplicantId)
			d.EmployerId, d.ResumeId = strings.ToUpper(d.EmployerId), strings.ToUpper(d.ResumeId)
		})
		repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
		repo.EXPECT().Record(gomock.Any(), fixtureEventID, fixtureRow()).Return(nil)
		require.NoError(t, rabbitmq.ApplicationHandler(repo)(context.Background(), e))
	})
}

// Anything malformed goes to the dead queue at once and is never stored.
func TestApplicationHandler_InvalidEventsArePermanent(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(*testing.T, *eventsv1.EventEnvelope)
	}{
		{"producer not application", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.Producer = "resume" }},
		{"wrong type", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.Type = "application.withdrawn" }},
		{"event_id not a UUID", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.EventId = "event-1" }},
		{"missing occurred_at", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"missing data", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"data of another event", func(t *testing.T, e *eventsv1.EventEnvelope) {
			packed, err := anypb.New(&eventsv1.ApplicationWithdrawn{ApplicationId: "11111111-1111-4111-8111-111111111111"})
			require.NoError(t, err)
			e.Data = packed
		}},
		{"application_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) {
			withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ApplicationId = "bad" })
		}},
		{"applicant_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) {
			withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ApplicantId = "" })
		}},
		{"employer_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) {
			withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.EmployerId = "employer" })
		}},
		{"resume_id not a UUID", func(t *testing.T, e *eventsv1.EventEnvelope) {
			withData(t, e, func(d *eventsv1.ApplicationSubmitted) { d.ResumeId = "resume" })
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := fixtureEnvelope(t)
			tt.edit(t, e)
			// No Record call is expected: gomock fails the test if one happens.
			repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
			err := rabbitmq.ApplicationHandler(repo)(context.Background(), e)
			require.ErrorIs(t, err, events.ErrPermanent)
		})
	}
	t.Run("nil envelope", func(t *testing.T) {
		repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
		require.ErrorIs(t, rabbitmq.ApplicationHandler(repo)(context.Background(), nil), events.ErrPermanent)
	})
}

// The repository decides between retry (database errors) and the dead queue
// (conflicting participants); the handler passes its result through unchanged.
func TestApplicationHandler_PassesRepositoryResultThrough(t *testing.T) {
	for _, tt := range []struct {
		name      string
		repoErr   error
		permanent bool
	}{
		{"database error is retried", errors.New("record application resume failed"), false},
		{"conflict goes to the dead queue", events.Permanent(errors.New("conflicting application resume")), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockApplicationResumeRepository(gomock.NewController(t))
			repo.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.repoErr)
			err := rabbitmq.ApplicationHandler(repo)(context.Background(), fixtureEnvelope(t))
			require.ErrorIs(t, err, tt.repoErr)
			require.Equal(t, tt.permanent, errors.Is(err, events.ErrPermanent))
		})
	}
}
