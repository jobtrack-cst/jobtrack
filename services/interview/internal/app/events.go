package app

import (
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
)

// InterviewScheduledRoutingKey is the event type and routing key (AGENTS.md Section 9).
const InterviewScheduledRoutingKey = "interview.scheduled"

// InterviewScheduledEvent builds interview.scheduled for a stored interview:
// scheduled_at in UTC, and occurred_at = the interview's created_at in UTC (JT-80).
func InterviewScheduledEvent(iv domain.Interview, eventID string) (*eventsv1.EventEnvelope, error) {
	if iv.ScheduledAt.IsZero() || iv.CreatedAt.IsZero() {
		return nil, errors.New("interview times are not set")
	}
	data, err := anypb.New(&eventsv1.InterviewScheduled{
		InterviewId: iv.InterviewID, ApplicationId: iv.ApplicationID, ScheduledAt: timestamppb.New(iv.ScheduledAt.UTC()),
	})
	if err != nil {
		return nil, err
	}
	return &eventsv1.EventEnvelope{
		EventId: eventID, Type: InterviewScheduledRoutingKey, Producer: "interview",
		OccurredAt: timestamppb.New(iv.CreatedAt.UTC()), Data: data,
	}, nil
}
