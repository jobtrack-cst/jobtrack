package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
// The entry errors are only used by RecordEntry; the Sprint 2 consumer
// decides how to handle an event that fails them.
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	// PERMISSION_DENIED
	ErrAdminRoleRequired = errors.New("only admins can list audit entries")
	// INVALID_ARGUMENT
	ErrInvalidEventID    = errors.New("event_id must be a UUID")
	ErrInvalidEventType  = errors.New("event_type must be a lowercase dot-separated routing key of 1 to 100 characters")
	ErrInvalidProducer   = errors.New("producer must be a kebab-case service name of 1 to 50 characters")
	ErrInvalidOccurredAt = errors.New("occurred_at is required")
	ErrInvalidDataJSON   = errors.New("data_json must be a valid UTF-8 JSON object of at most 65536 bytes")
	ErrInvalidTimestamp  = errors.New("start_time and end_time must be valid timestamps")
	ErrInvalidTimeRange  = errors.New("start_time must be before end_time")
	ErrInvalidPageSize   = errors.New("page_size must not be negative")
	ErrInvalidPageToken  = errors.New("page_token is not valid")
)
