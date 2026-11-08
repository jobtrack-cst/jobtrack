package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
// The outcome event errors are only used by RecordEvent; the Sprint 2
// consumer decides how to handle an event that fails them.
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	// PERMISSION_DENIED
	ErrAdminRoleRequired = errors.New("only admins can view analytics")
	// INVALID_ARGUMENT
	ErrInvalidTimestamp  = errors.New("start_time and end_time are required and must be valid timestamps")
	ErrInvalidTimeRange  = errors.New("start_time must be before end_time")
	ErrInvalidEventID    = errors.New("event_id must be a UUID")
	ErrInvalidEventType  = errors.New("event type is not one that analytics counts")
	ErrInvalidStatus     = errors.New("new_status must be 1 to 32 characters for application.status_changed and empty for other events")
	ErrInvalidOccurredAt = errors.New("occurred_at is required")
)
