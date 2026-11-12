package domain

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits agreed in JT-52. The migration enforces the details length too.
const (
	MaxDetailsLength = 1000
	MaxScheduleAhead = 365 * 24 * time.Hour
)

// ApplicationParticipants is the trusted local projection of
// application.submitted (filled by the Sprint 2 consumer), the same design as
// Review, Offer and Messaging. IDs are user IDs from auth.
type ApplicationParticipants struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
}

// CheckOwningEmployer allows only the employer stored for the application.
func (p ApplicationParticipants) CheckOwningEmployer(c Caller) error {
	if c.Role != RoleEmployer || c.UserID != p.EmployerID {
		return ErrNotOwningEmployer
	}
	return nil
}

// CheckReader allows the application's applicant (as an applicant) or its
// employer (as an employer).
func (p ApplicationParticipants) CheckReader(c Caller) error {
	switch {
	case c.Role == RoleApplicant && c.UserID == p.ApplicantID:
		return nil
	case c.Role == RoleEmployer && c.UserID == p.EmployerID:
		return nil
	default:
		return ErrNotParticipant
	}
}

// Interview is one scheduled interview. An application may have several,
// but not two at the same time.
type Interview struct {
	InterviewID   string
	ApplicationID string
	EmployerID    string
	ApplicantID   string
	ScheduledAt   time.Time
	Details       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ScheduleInput is the raw content of a ScheduleInterview request.
type ScheduleInput struct {
	ApplicationID string
	ScheduledAt   time.Time
	Details       string
}

// ValidSchedule is a checked request: normalized ID, UTC time truncated to
// the microseconds Postgres stores, trimmed details.
type ValidSchedule struct {
	ApplicationID string
	ScheduledAt   time.Time
	Details       string
}

// NewSchedule applies the JT-52 rules. now is the current time from the clock.
func NewSchedule(in ScheduleInput, now time.Time) (ValidSchedule, error) {
	applicationID, err := NormalizeApplicationID(in.ApplicationID)
	if err != nil {
		return ValidSchedule{}, err
	}
	if in.ScheduledAt.IsZero() {
		return ValidSchedule{}, ErrScheduledAtMissing
	}
	at := in.ScheduledAt.UTC().Truncate(time.Microsecond)
	if !at.After(now) {
		return ValidSchedule{}, ErrScheduledAtNotFuture
	}
	if at.After(now.Add(MaxScheduleAhead)) {
		return ValidSchedule{}, ErrScheduledAtTooFar
	}
	details, ok := cleanDetails(in.Details)
	if !ok {
		return ValidSchedule{}, ErrInvalidDetails
	}
	return ValidSchedule{ApplicationID: applicationID, ScheduledAt: at, Details: details}, nil
}

// cleanDetails trims the text and allows line breaks and tabs, but no other
// control characters or invalid UTF-8.
func cleanDetails(s string) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", false
		}
	}
	s = strings.TrimSpace(s)
	return s, utf8.RuneCountInString(s) <= MaxDetailsLength
}
