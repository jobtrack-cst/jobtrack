package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	// PERMISSION_DENIED
	ErrEmployerRoleRequired = errors.New("only employers can manage a company profile")
	// INVALID_ARGUMENT
	ErrInvalidEmployerID  = errors.New("employer_id must be a UUID")
	ErrInvalidCompanyName = errors.New("company_name must be 1 to 150 characters")
	ErrInvalidDescription = errors.New("description must be at most 2000 characters")
	ErrInvalidWebsite     = errors.New("website must be an absolute http or https URL of at most 255 characters")
	ErrInvalidLocation    = errors.New("location must be at most 100 characters")
	// NOT_FOUND
	ErrProfileNotFound = errors.New("company profile not found")
	// ALREADY_EXISTS
	ErrProfileExists = errors.New("company profile already exists for this employer")
)
