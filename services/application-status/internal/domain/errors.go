package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrRoleNotAllowed = errors.New("only applicants and employers can read application status")
	ErrNotParticipant = errors.New("only the application's applicant or employer can read its status")

	// INVALID_ARGUMENT
	ErrInvalidApplicationID = errors.New("application_id must be a UUID")
	ErrInvalidParticipantID = errors.New("applicant_id and employer_id must be UUIDs")
	ErrInvalidTime          = errors.New("transition time is required")
	ErrInvalidPageSize      = errors.New("page_size must not be negative")
	ErrInvalidPageToken     = errors.New("page_token must be an offset as a decimal string")

	// NOT_FOUND
	ErrApplicationNotFound = errors.New("application status not found")

	// ALREADY_EXISTS
	ErrApplicationExists = errors.New("application status already recorded")

	// FAILED_PRECONDITION
	ErrInvalidTransition = errors.New("status transition is not allowed")
	// ErrStatusChanged is returned by the repository when a conditional update
	// finds the application no longer in the expected status (a concurrent change).
	ErrStatusChanged = errors.New("the application status changed during the request")
)
