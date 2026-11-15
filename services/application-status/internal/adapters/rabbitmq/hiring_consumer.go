package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

// HiringQueue is the second application-status queue, for the review,
// interview and offer events (JT-81).
const HiringQueue = "application-status.hiring"

// HiringBindings are the five events the hiring consumer reads.
var HiringBindings = []string{"review.completed", "interview.scheduled", "offer.made", "offer.accepted", "offer.declined"}

func HiringConsumerConfig(url string, logger *slog.Logger, repo ports.HiringEventRepository) events.ConsumerConfig {
	return events.ConsumerConfig{URL: url, Queue: HiringQueue, Bindings: HiringBindings, Logger: logger, Handler: HiringHandler(repo)}
}

// HiringHandler sends events that can never apply (invalid input, a
// transition Section 6 does not allow, a different applicant) to the dead
// queue at once, so they never block the queue. An unknown application or a
// database error is retried under the JT-61 policy.
func HiringHandler(repo ports.HiringEventRepository) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		input, err := HiringFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		err = repo.ApplyHiringEvent(ctx, input)
		if errors.Is(err, domain.ErrConflictingEvent) || errors.Is(err, domain.ErrInvalidTransition) {
			return events.Permanent(err)
		}
		return err
	}
}

// hiringSource is the producer and payload type each routing key must have.
var hiringSource = map[string]struct {
	producer string
	data     func() proto.Message
}{
	"review.completed":    {"review", func() proto.Message { return new(eventsv1.ReviewCompleted) }},
	"interview.scheduled": {"interview", func() proto.Message { return new(eventsv1.InterviewScheduled) }},
	"offer.made":          {"offer", func() proto.Message { return new(eventsv1.OfferMade) }},
	"offer.accepted":      {"offer", func() proto.Message { return new(eventsv1.OfferAccepted) }},
	"offer.declined":      {"offer", func() proto.Message { return new(eventsv1.OfferDeclined) }},
}

// HiringFromEnvelope validates one hiring event: the producer matching its
// type, a UUID event_id, a valid nonzero occurred_at, the matching payload,
// UUID IDs (stored lowercase) and, for reviews, decision SHORTLISTED or REJECTED.
func HiringFromEnvelope(e *eventsv1.EventEnvelope) (domain.HiringEvent, error) {
	bad := domain.ErrConflictingEvent
	src, ok := hiringSource[e.GetType()]
	if !ok || e.GetProducer() != src.producer {
		return domain.HiringEvent{}, bad
	}
	if e.GetOccurredAt() == nil || e.GetOccurredAt().CheckValid() != nil || e.GetOccurredAt().AsTime().IsZero() {
		return domain.HiringEvent{}, bad
	}
	eventID, ok := canonicalUUID(e.GetEventId())
	if !ok {
		return domain.HiringEvent{}, bad
	}
	data := src.data()
	if e.GetData() == nil || e.GetData().UnmarshalTo(data) != nil {
		return domain.HiringEvent{}, bad
	}
	v := domain.HiringEvent{EventID: eventID, OccurredAt: e.GetOccurredAt().AsTime().UTC()}
	var ids []string
	switch d := data.(type) {
	case *eventsv1.ReviewCompleted:
		switch d.GetDecision() {
		case "SHORTLISTED":
			v.Kind = domain.HiringReviewShortlisted
		case "REJECTED":
			v.Kind = domain.HiringReviewRejected
		default:
			return domain.HiringEvent{}, bad
		}
		ids = []string{d.GetApplicationId()}
	case *eventsv1.InterviewScheduled:
		if d.GetScheduledAt() == nil || d.GetScheduledAt().CheckValid() != nil {
			return domain.HiringEvent{}, bad
		}
		v.Kind = domain.HiringInterviewScheduled
		ids = []string{d.GetApplicationId(), d.GetInterviewId()}
	case *eventsv1.OfferMade:
		v.Kind = domain.HiringOfferMade
		ids = []string{d.GetApplicationId(), d.GetOfferId(), d.GetApplicantId()}
	case *eventsv1.OfferAccepted:
		v.Kind = domain.HiringOfferAccepted
		ids = []string{d.GetApplicationId(), d.GetOfferId(), d.GetApplicantId()}
	case *eventsv1.OfferDeclined:
		v.Kind = domain.HiringOfferDeclined
		ids = []string{d.GetApplicationId(), d.GetOfferId(), d.GetApplicantId()}
	}
	for i, id := range ids {
		canonical, ok := canonicalUUID(id)
		if !ok {
			return domain.HiringEvent{}, bad
		}
		ids[i] = canonical
	}
	v.ApplicationID = ids[0]
	if len(ids) == 3 {
		v.ApplicantID = ids[2]
	}
	return v, nil
}

// canonicalUUID accepts only the 36-character form, as JT-60 does, and returns it lowercase.
func canonicalUUID(s string) (string, bool) {
	id, err := uuid.Parse(s)
	if err != nil || len(s) != 36 {
		return "", false
	}
	return id.String(), true
}
