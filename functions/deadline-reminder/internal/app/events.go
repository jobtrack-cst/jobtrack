// Package app runs one deadline-reminder pass.
package app

import (
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

// ReminderRoutingKey is the event type and routing key (AGENTS.md Section 9).
const ReminderRoutingKey = "deadline.reminder"

// ReminderEvent builds deadline.reminder for one saver of one job. The
// event_id is the reminder's stable ID; occurred_at is the run time in UTC.
func ReminderEvent(r domain.Reminder, runAt time.Time) (*eventsv1.EventEnvelope, error) {
	data, err := anypb.New(&eventsv1.DeadlineReminder{
		JobId: r.JobID, ApplicantId: r.ApplicantID, Deadline: timestamppb.New(r.Deadline.UTC()),
	})
	if err != nil {
		return nil, err
	}
	return &eventsv1.EventEnvelope{
		EventId: r.EventID(), Type: ReminderRoutingKey, Producer: "deadline-reminder",
		OccurredAt: timestamppb.New(runAt.UTC().Truncate(time.Microsecond)), Data: data,
	}, nil
}
