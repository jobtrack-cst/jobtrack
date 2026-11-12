package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrEmployerRoleRequired = errors.New("only employers can complete or view reviews")
	ErrNotOwningEmployer    = errors.New("only the employer that owns the application's job can review it")

	// INVALID_ARGUMENT
	ErrInvalidApplicationID = errors.New("application_id must be a UUID")
	ErrInvalidScore         = errors.New("score must be from 1 to 5")
	ErrInvalidDecision      = errors.New("decision must be SHORTLISTED or REJECTED")

	// NOT_FOUND: unknown applications include those whose application.submitted
	// has not been processed yet, as in Messaging and Offer.
	ErrApplicationNotFound = errors.New("application not found")
	ErrReviewNotFound      = errors.New("review not found")

	// ALREADY_EXISTS
	ErrReviewExists = errors.New("application already has a completed review")
)
