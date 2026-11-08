package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	// PERMISSION_DENIED
	ErrEmployerRoleRequired = errors.New("only employers can submit or list their own verification requests")
	ErrAdminRoleRequired    = errors.New("only admins can list or decide verification requests")
	// INVALID_ARGUMENT
	ErrInvalidRequestID          = errors.New("request_id must be a UUID")
	ErrInvalidRegistrationNumber = errors.New("registration_number must be 1 to 50 characters")
	ErrInvalidNotes              = errors.New("notes must be at most 1000 characters")
	ErrInvalidRejectionReason    = errors.New("reason must be 1 to 500 characters")
	ErrInvalidStatusFilter       = errors.New("status filter is not a known request status")
	ErrInvalidPageSize           = errors.New("page_size must not be negative")
	ErrInvalidPageToken          = errors.New("page_token is not valid")
	// NOT_FOUND
	ErrRequestNotFound = errors.New("verification request not found")
	// ALREADY_EXISTS
	ErrPendingRequestExists = errors.New("employer already has a pending verification request")
	// FAILED_PRECONDITION
	ErrAlreadyVerified   = errors.New("employer is already verified")
	ErrRequestNotPending = errors.New("verification request has already been decided")
)
