package domain

import (
	"errors"
	"time"
)

// OutgoingEvent retains the original serialized envelope and temporary ownership.
// ClaimToken changes on reclamation; ID and Payload never change.
type OutgoingEvent struct {
	ID         string
	Type       string
	Payload    string
	ClaimToken string
	LeaseUntil time.Time
}

var (
	ErrInvalidOutboxLease      = errors.New("outbox lease must be positive whole microseconds")
	ErrInvalidOutboxRetryDelay = errors.New("outbox retry delay must be nonnegative whole microseconds")
	ErrInvalidOutboxClaim      = errors.New("outbox event ID and claim token must be UUIDs")
)

// PostgreSQL stores timestamps at microsecond precision. Refuse truncation.
func ValidateOutboxLease(lease time.Duration) error {
	if lease < time.Microsecond || lease%time.Microsecond != 0 {
		return ErrInvalidOutboxLease
	}
	return nil
}

func ValidateOutboxRetryDelay(delay time.Duration) error {
	if delay < 0 || delay%time.Microsecond != 0 {
		return ErrInvalidOutboxRetryDelay
	}
	return nil
}

func ValidateOutboxClaim(eventID, token string) error {
	if _, ok := normalizeUUID(eventID); !ok {
		return ErrInvalidOutboxClaim
	}
	if _, ok := normalizeUUID(token); !ok {
		return ErrInvalidOutboxClaim
	}
	return nil
}
