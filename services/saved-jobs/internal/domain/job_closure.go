package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// ErrInvalidJobClosedEvent marks a job.closed event that can never be applied.
var ErrInvalidJobClosedEvent = errors.New("invalid job.closed event")

// Only the canonical lowercase form, so the same job always gets the same key.
var closureUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// JobClosure is a validated job.closed event from job-posting.
type JobClosure struct {
	EventID  string
	JobID    string
	ClosedAt time.Time
}

// ClosureOutcome reports what applying a JobClosure changed.
type ClosureOutcome struct {
	Duplicate        bool // the event ID was already processed; nothing changed
	RemovedBookmarks int
}

// NewJobClosure stores IDs lowercase and the time in UTC.
func NewJobClosure(eventID, jobID string, closedAt time.Time) (JobClosure, error) {
	eventID, jobID = strings.ToLower(eventID), strings.ToLower(jobID)
	if !closureUUID.MatchString(eventID) || !closureUUID.MatchString(jobID) || closedAt.IsZero() {
		return JobClosure{}, ErrInvalidJobClosedEvent
	}
	return JobClosure{EventID: eventID, JobID: jobID, ClosedAt: closedAt.UTC()}, nil
}
