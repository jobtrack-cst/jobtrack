package rabbitmq_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

const (
	fxApplication = "11111111-1111-4111-8111-111111111111"
	fxApplicant   = "22222222-2222-4222-8222-222222222222"
)

func hiringFixture(t *testing.T, name string) *eventsv1.EventEnvelope {
	t.Helper()
	b, err := os.ReadFile("../../../../../tests/contract/fixtures/" + name + ".json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	return e
}

type hiringRepo struct {
	got []domain.HiringEvent
	err error
}

func (r *hiringRepo) ApplyHiringEvent(_ context.Context, e domain.HiringEvent) error {
	r.got = append(r.got, e)
	return r.err
}

func TestHiringConsumerConfig(t *testing.T) {
	cfg := rabbitmq.HiringConsumerConfig("amqp://example", slog.New(slog.NewTextHandler(io.Discard, nil)), &hiringRepo{})
	assert.Equal(t, "application-status.hiring", cfg.Queue)
	assert.Equal(t, []string{"review.completed", "interview.scheduled", "offer.made", "offer.accepted", "offer.declined"}, cfg.Bindings)
	assert.NotNil(t, cfg.Handler)
}

// Every shared fixture is read into the expected domain event.
func TestHiringFromEnvelope_SharedFixtures(t *testing.T) {
	for name, want := range map[string]domain.HiringEvent{
		"review.completed":    {EventID: "77777777-7777-4777-8777-777777777791", ApplicationID: fxApplication, Kind: domain.HiringReviewShortlisted},
		"interview.scheduled": {EventID: "77777777-7777-4777-8777-777777777801", ApplicationID: fxApplication, Kind: domain.HiringInterviewScheduled},
		"offer.made":          {EventID: "77777777-7777-4777-8777-777777777771", ApplicationID: fxApplication, ApplicantID: fxApplicant, Kind: domain.HiringOfferMade},
		"offer.accepted":      {EventID: "77777777-7777-4777-8777-777777777772", ApplicationID: fxApplication, ApplicantID: fxApplicant, Kind: domain.HiringOfferAccepted},
		"offer.declined":      {EventID: "77777777-7777-4777-8777-777777777773", ApplicationID: fxApplication, ApplicantID: fxApplicant, Kind: domain.HiringOfferDeclined},
	} {
		t.Run(name, func(t *testing.T) {
			e := hiringFixture(t, name)
			got, err := rabbitmq.HiringFromEnvelope(e)
			require.NoError(t, err)
			want.OccurredAt = e.GetOccurredAt().AsTime()
			assert.Equal(t, want, got)
			repo := &hiringRepo{}
			require.NoError(t, rabbitmq.HiringHandler(repo)(context.Background(), e))
			assert.Equal(t, []domain.HiringEvent{want}, repo.got)
		})
	}
}

func repack(t *testing.T, e *eventsv1.EventEnvelope, m proto.Message) *eventsv1.EventEnvelope {
	t.Helper()
	packed, err := anypb.New(m)
	require.NoError(t, err)
	c := proto.Clone(e).(*eventsv1.EventEnvelope)
	c.Data = packed
	return c
}

func TestHiringFromEnvelope_RejectedDecisionAndUppercaseIDs(t *testing.T) {
	e := repack(t, hiringFixture(t, "review.completed"), &eventsv1.ReviewCompleted{ApplicationId: strings.ToUpper(fxApplication), Decision: "REJECTED"})
	e.EventId = strings.ToUpper(e.EventId)
	got, err := rabbitmq.HiringFromEnvelope(e)
	require.NoError(t, err)
	assert.Equal(t, domain.HiringReviewRejected, got.Kind)
	assert.Equal(t, fxApplication, got.ApplicationID)
	assert.Equal(t, "77777777-7777-4777-8777-777777777791", got.EventID)

	o := repack(t, hiringFixture(t, "offer.made"), &eventsv1.OfferMade{OfferId: "88888888-8888-4888-8888-888888888888",
		ApplicationId: fxApplication, ApplicantId: strings.ToUpper(fxApplicant)})
	got, err = rabbitmq.HiringFromEnvelope(o)
	require.NoError(t, err)
	assert.Equal(t, fxApplicant, got.ApplicantID, "compared with the stored lowercase applicant")
}

func TestHiringHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	cases := map[string]func(t *testing.T) *eventsv1.EventEnvelope{
		"nil envelope": func(*testing.T) *eventsv1.EventEnvelope { return nil },
		"unknown type": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.made")
			e.Type = "offer.expired"
			return e
		},
		"application event on this queue": func(t *testing.T) *eventsv1.EventEnvelope { return hiringFixture(t, "application.submitted") },
		"review from wrong producer": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "review.completed")
			e.Producer = "interview"
			return e
		},
		"offer from wrong producer": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.accepted")
			e.Producer = "application"
			return e
		},
		"event_id not a UUID": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "interview.scheduled")
			e.EventId = "x"
			return e
		},
		"event_id without dashes": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "interview.scheduled")
			e.EventId = strings.ReplaceAll(e.EventId, "-", "")
			return e
		},
		"missing occurred_at": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.made")
			e.OccurredAt = nil
			return e
		},
		"zero occurred_at": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.made")
			e.OccurredAt = timestamppb.New(time.Time{})
			return e
		},
		"invalid occurred_at": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.made")
			e.OccurredAt = &timestamppb.Timestamp{Seconds: 1, Nanos: -1}
			return e
		},
		"missing data": func(t *testing.T) *eventsv1.EventEnvelope {
			e := hiringFixture(t, "offer.declined")
			e.Data = nil
			return e
		},
		"data of another event": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "offer.accepted"), &eventsv1.OfferDeclined{})
		},
		"unknown decision": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "review.completed"), &eventsv1.ReviewCompleted{ApplicationId: fxApplication, Decision: "shortlisted"})
		},
		"review application_id not a UUID": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "review.completed"), &eventsv1.ReviewCompleted{ApplicationId: "x", Decision: "SHORTLISTED"})
		},
		"interview_id not a UUID": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "interview.scheduled"), &eventsv1.InterviewScheduled{InterviewId: "1", ApplicationId: fxApplication, ScheduledAt: timestamppb.Now()})
		},
		"missing scheduled_at": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "interview.scheduled"), &eventsv1.InterviewScheduled{InterviewId: "88888888-8888-4888-8888-888888888801", ApplicationId: fxApplication})
		},
		"offer_id not a UUID": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "offer.made"), &eventsv1.OfferMade{OfferId: "1", ApplicationId: fxApplication, ApplicantId: fxApplicant})
		},
		"offer applicant_id missing": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "offer.accepted"), &eventsv1.OfferAccepted{OfferId: "88888888-8888-4888-8888-888888888888", ApplicationId: fxApplication})
		},
		"offer application_id missing": func(t *testing.T) *eventsv1.EventEnvelope {
			return repack(t, hiringFixture(t, "offer.declined"), &eventsv1.OfferDeclined{OfferId: "88888888-8888-4888-8888-888888888888", ApplicantId: fxApplicant})
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &hiringRepo{}
			err := rabbitmq.HiringHandler(repo)(context.Background(), build(t))
			require.ErrorIs(t, err, events.ErrPermanent)
			assert.Empty(t, repo.got, "nothing reaches the repository")
		})
	}
}

func TestHiringHandler_RepositoryErrors(t *testing.T) {
	for _, tt := range []struct {
		name      string
		err       error
		permanent bool
	}{
		{"invalid transition is dead-lettered", domain.ErrInvalidTransition, true},
		{"conflicting applicant is dead-lettered", domain.ErrConflictingEvent, true},
		{"unknown application is retried", domain.ErrApplicationNotFound, false},
		{"concurrent change is retried", domain.ErrStatusChanged, false},
		{"database error is retried", errors.New("apply hiring event failed"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := rabbitmq.HiringHandler(&hiringRepo{err: tt.err})(context.Background(), hiringFixture(t, "review.completed"))
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.permanent, errors.Is(err, events.ErrPermanent))
		})
	}
}
