package app

import (
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

// ReviewCompletedRoutingKey is the event type and routing key (AGENTS.md Section 9).
const ReviewCompletedRoutingKey = "review.completed"

// ReviewCompletedEvent builds review.completed for a stored review. The
// decision is SHORTLISTED or REJECTED and occurred_at is the review's
// created_at in UTC (JT-79).
func ReviewCompletedEvent(rv domain.Review, eventID string) (*eventsv1.EventEnvelope, error) {
	decision, ok := domain.ParseDecision(rv.Decision.String())
	if !ok {
		return nil, errors.New("review decision is not valid")
	}
	data, err := anypb.New(&eventsv1.ReviewCompleted{ApplicationId: rv.ApplicationID, Decision: decision.String()})
	if err != nil {
		return nil, err
	}
	return &eventsv1.EventEnvelope{
		EventId: eventID, Type: ReviewCompletedRoutingKey, Producer: "review",
		OccurredAt: timestamppb.New(rv.CreatedAt.UTC()), Data: data,
	}, nil
}
