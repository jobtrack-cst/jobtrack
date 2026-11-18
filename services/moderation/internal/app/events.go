package app

import (
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

// Producer is the envelope producer for every event this service publishes.
// job-posting's consumer (JT-68) accepts moderation.job_removed only from it.
const Producer = "moderation"

// EventJobRemoved is the routing key of ModerationJobRemoved in events.proto.
const EventJobRemoved = "moderation.job_removed"

// JobRemovedEvent builds the event for a takedown. The contract carries only
// the job ID and the reason; the admin stays in removed_jobs (JT-34).
// occurred_at is the removal time.
func JobRemovedEvent(rm domain.Removal, eventID string) (*eventsv1.EventEnvelope, error) {
	if rm.JobID == "" || rm.Reason == "" || rm.RemovedAt.IsZero() {
		return nil, errors.New("removal is incomplete")
	}
	payload, err := anypb.New(&eventsv1.ModerationJobRemoved{JobId: rm.JobID, Reason: rm.Reason})
	if err != nil {
		return nil, errors.New("encode removal event failed")
	}
	return &eventsv1.EventEnvelope{
		EventId:    eventID,
		Type:       EventJobRemoved,
		OccurredAt: timestamppb.New(rm.RemovedAt.UTC()),
		Producer:   Producer,
		Data:       payload,
	}, nil
}
