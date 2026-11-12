package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// MaxRejectionReasonLength matches the company-verification reject reason
// limit (JT-33), so a valid rejection always fits.
const MaxRejectionReasonLength = 500

// VerificationDecision is one employer.verified or
// employer.verification_rejected event, already decoded (JT-56).
type VerificationDecision struct {
	EventID         string
	EmployerID      string
	Status          VerificationStatus
	RejectionReason string
	// DecidedAt is the envelope occurred_at. A decision only replaces an
	// older one, so events that arrive out of order cannot undo a newer one.
	DecidedAt time.Time
}

// VerificationOutcome tells the consumer what applying a decision did.
type VerificationOutcome uint8

const (
	// OutcomeApplied: stored and copied to the company profile.
	OutcomeApplied VerificationOutcome = iota + 1
	// OutcomeStoredWithoutProfile: stored; Create copies it into the profile.
	OutcomeStoredWithoutProfile
	// OutcomeDuplicate: this event_id was already processed.
	OutcomeDuplicate
	// OutcomeStale: a newer decision is already stored, so nothing changed.
	OutcomeStale
)

func (o VerificationOutcome) String() string {
	switch o {
	case OutcomeApplied:
		return "applied"
	case OutcomeStoredWithoutProfile:
		return "stored_without_profile"
	case OutcomeDuplicate:
		return "duplicate"
	case OutcomeStale:
		return "stale"
	default:
		return "unknown"
	}
}

// NewVerificationDecision validates a decoded event. The reason is trimmed,
// required for a rejection and must be empty for an approval.
func NewVerificationDecision(eventID, employerID string, status VerificationStatus, reason string, decidedAt time.Time) (VerificationDecision, error) {
	if !uuidPattern.MatchString(eventID) {
		return VerificationDecision{}, ErrInvalidVerificationEvent
	}
	id, err := NormalizeEmployerID(employerID)
	if err != nil {
		return VerificationDecision{}, ErrInvalidVerificationEvent
	}
	reason = strings.TrimSpace(reason)
	switch status {
	case VerificationVerified:
		if reason != "" {
			return VerificationDecision{}, ErrInvalidVerificationEvent
		}
	case VerificationRejected:
		if n := utf8.RuneCountInString(reason); n < 1 || n > MaxRejectionReasonLength || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) {
			return VerificationDecision{}, ErrInvalidVerificationEvent
		}
	default:
		return VerificationDecision{}, ErrInvalidVerificationEvent
	}
	if decidedAt.IsZero() {
		return VerificationDecision{}, ErrInvalidVerificationEvent
	}
	return VerificationDecision{
		EventID:         strings.ToLower(eventID),
		EmployerID:      id,
		Status:          status,
		RejectionReason: reason,
		DecidedAt:       decidedAt.UTC(),
	}, nil
}
