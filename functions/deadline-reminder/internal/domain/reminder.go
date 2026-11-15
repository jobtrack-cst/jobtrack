// Package domain holds the deadline-reminder window and the stable event ID
// that keeps repeated runs from sending duplicates (JT-82).
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Window is how far ahead a run looks: jobs closing within 48 hours.
const Window = 48 * time.Hour

// eventIDNamespace is fixed so every run derives the same event_id for the
// same saver, job and deadline. Changing it would resend every reminder.
var eventIDNamespace = uuid.MustParse("6a1f3c2e-5d4b-4f8a-9c7e-2b1d0e9f8a71")

// ErrInvalidReminder means a job or saver ID from an upstream service is not a UUID,
// or the deadline is missing.
var ErrInvalidReminder = errors.New("reminder needs UUID job and applicant IDs and a deadline")

// Period returns the window one run covers: [now, now+48h), in UTC truncated
// to the microsecond, as job-posting stores deadlines.
func Period(now time.Time) (start, end time.Time) {
	start = now.UTC().Truncate(time.Microsecond)
	return start, start.Add(Window)
}

// Reminder is one deadline.reminder: one saver of one job.
type Reminder struct {
	JobID       string
	ApplicantID string
	Deadline    time.Time
}

// NewReminder validates the IDs, stores them lowercase and the deadline in UTC
// truncated to the microsecond.
func NewReminder(jobID, applicantID string, deadline time.Time) (Reminder, error) {
	job, err := uuid.Parse(jobID)
	if err != nil || len(jobID) != 36 {
		return Reminder{}, ErrInvalidReminder
	}
	applicant, err := uuid.Parse(applicantID)
	if err != nil || len(applicantID) != 36 {
		return Reminder{}, ErrInvalidReminder
	}
	if deadline.IsZero() {
		return Reminder{}, ErrInvalidReminder
	}
	return Reminder{JobID: job.String(), ApplicantID: applicant.String(), Deadline: deadline.UTC().Truncate(time.Microsecond)}, nil
}

// EventID is a UUIDv5 of the job, the applicant and the deadline. Every run
// that sees the same saver, job and deadline publishes the same event_id, so
// consumers ignore the repeat (AGENTS.md Section 9). A changed deadline is a
// new reminder.
func (r Reminder) EventID() string {
	name := r.JobID + "/" + r.ApplicantID + "/" + r.Deadline.UTC().Format(time.RFC3339Nano)
	return uuid.NewSHA1(eventIDNamespace, []byte(name)).String()
}

// ClosingJob is one published job whose deadline falls inside the window.
type ClosingJob struct {
	ID       string
	Deadline time.Time
}
