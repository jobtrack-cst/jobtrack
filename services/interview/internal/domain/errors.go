package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrEmployerRoleRequired = errors.New("only employers can schedule interviews")
	ErrRoleNotAllowed       = errors.New("only applicants and employers can read interviews")
	ErrNotOwningEmployer    = errors.New("only the employer that owns the application's job can schedule its interviews")
	ErrNotParticipant       = errors.New("only the application's applicant or employer can read its interviews")

	// INVALID_ARGUMENT
	ErrInvalidApplicationID = errors.New("application_id must be a UUID")
	ErrInvalidInterviewID   = errors.New("interview_id must be a UUID")
	ErrScheduledAtMissing   = errors.New("scheduled_at is required")
	ErrScheduledAtNotFuture = errors.New("scheduled_at must be in the future")
	ErrScheduledAtTooFar    = errors.New("scheduled_at must be at most 365 days ahead")
	ErrInvalidDetails       = errors.New("details must be at most 1000 characters of valid text")
	ErrInvalidPageSize      = errors.New("page_size must not be negative")
	ErrInvalidPageToken     = errors.New("page_token must be an offset as a decimal string")

	// NOT_FOUND: unknown applications include those whose application.submitted
	// has not been processed yet, as in Messaging, Offer and Review.
	ErrApplicationNotFound = errors.New("application not found")
	ErrInterviewNotFound   = errors.New("interview not found")

	// ALREADY_EXISTS
	ErrInterviewExists = errors.New("the application already has an interview at that time")
)
