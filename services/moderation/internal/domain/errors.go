package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	// PERMISSION_DENIED
	ErrAdminRoleRequired = errors.New("only admins can list reports, dismiss reports or remove jobs")
	// INVALID_ARGUMENT
	ErrInvalidJobID         = errors.New("job_id must be a UUID")
	ErrInvalidReportID      = errors.New("report_id must be a UUID")
	ErrInvalidReportReason  = errors.New("reason must be 1 to 1000 characters")
	ErrInvalidRemovalReason = errors.New("reason must be 1 to 500 characters")
	ErrInvalidStatusFilter  = errors.New("status filter is not a known report status")
	ErrInvalidPageSize      = errors.New("page_size must not be negative")
	ErrInvalidPageToken     = errors.New("page_token is not valid")
	// NOT_FOUND
	ErrReportNotFound = errors.New("report not found")
	// ALREADY_EXISTS
	ErrOpenReportExists = errors.New("you already have an open report for this job")
	// FAILED_PRECONDITION
	ErrJobAlreadyRemoved = errors.New("job has already been removed")
	ErrReportNotOpen     = errors.New("report is no longer open")
)
