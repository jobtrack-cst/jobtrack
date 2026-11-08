package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// The eight event types analytics counts (AGENTS.md Section 4).
const (
	EventJobPublished         = "job.published"
	EventJobClosed            = "job.closed"
	EventApplicationSubmitted = "application.submitted"
	EventApplicationWithdrawn = "application.withdrawn"
	EventStatusChanged        = "application.status_changed"
	EventOfferMade            = "offer.made"
	EventOfferAccepted        = "offer.accepted"
	EventOfferDeclined        = "offer.declined"
)

// MaxStatusLength is in characters after trimming, noted in the JT-36 PR.
// The longest lifecycle status today, UNDER_REVIEW, has 12.
const MaxStatusLength = 32

// IsCountedEventType reports whether analytics stores events of this type.
// Values are not trimmed: a routing key with spaces is wrong, not untidy.
func IsCountedEventType(s string) bool {
	switch s {
	case EventJobPublished, EventJobClosed, EventApplicationSubmitted, EventApplicationWithdrawn,
		EventStatusChanged, EventOfferMade, EventOfferAccepted, EventOfferDeclined:
		return true
	}
	return false
}

// OutcomeEvent is one counted event. Only the fields the summary needs are
// kept: no job, employer or applicant IDs.
type OutcomeEvent struct {
	EventID   string
	EventType string
	// NewStatus is set only for application.status_changed.
	NewStatus  string
	OccurredAt time.Time
}

// NewOutcomeEvent validates an event before RecordEvent stores it.
func NewOutcomeEvent(eventID, eventType, newStatus string, occurredAt time.Time) (OutcomeEvent, error) {
	id, err := NormalizeID(eventID, ErrInvalidEventID)
	if err != nil {
		return OutcomeEvent{}, err
	}
	if !IsCountedEventType(eventType) {
		return OutcomeEvent{}, ErrInvalidEventType
	}
	status, err := validateStatus(eventType, newStatus)
	if err != nil {
		return OutcomeEvent{}, err
	}
	if occurredAt.IsZero() {
		return OutcomeEvent{}, ErrInvalidOccurredAt
	}
	return OutcomeEvent{EventID: id, EventType: eventType, NewStatus: status, OccurredAt: occurredAt.UTC()}, nil
}

// validateStatus stores the status as sent in the event, only trimmed. It is
// not checked against the lifecycle list, so a new status needs no
// analytics change. Invalid UTF-8 and NUL would be refused by Postgres as
// INTERNAL errors.
func validateStatus(eventType, raw string) (string, error) {
	if eventType != EventStatusChanged {
		if raw != "" {
			return "", ErrInvalidStatus
		}
		return "", nil
	}
	s := strings.TrimSpace(raw)
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", ErrInvalidStatus
	}
	if n := utf8.RuneCountInString(s); n < 1 || n > MaxStatusLength {
		return "", ErrInvalidStatus
	}
	return s, nil
}
