package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrEmployerRoleRequired = errors.New("only employers can manage jobs")
	ErrNotJobOwner          = errors.New("only the employer who owns this job can change it")

	// INVALID_ARGUMENT
	ErrInvalidJobID        = errors.New("job_id must be a UUID")
	ErrInvalidTitle        = errors.New("title must be 1 to 150 characters")
	ErrInvalidDescription  = errors.New("description must be at most 5000 characters")
	ErrInvalidCategoryID   = errors.New("category_id must be empty or a UUID")
	ErrInvalidTagIDs       = errors.New("tag_ids must be at most 20 UUIDs")
	ErrInvalidLocation     = errors.New("location must be at most 100 characters")
	ErrInvalidDeadline     = errors.New("deadline is required and must be in the future")
	ErrInvalidStatusFilter = errors.New("status filter is not a known job status")
	ErrInvalidTimeRange    = errors.New("start_time and end_time are required and start_time must be before end_time")
	ErrInvalidPageSize     = errors.New("page_size must not be negative")
	ErrInvalidPageToken    = errors.New("page_token must be an offset as a decimal string")

	// NOT_FOUND
	ErrJobNotFound = errors.New("job not found")

	// FAILED_PRECONDITION
	ErrJobNotEditable      = errors.New("only DRAFT or PUBLISHED jobs can be edited")
	ErrJobNotDraft         = errors.New("only DRAFT jobs can be published")
	ErrJobNotPublished     = errors.New("only PUBLISHED jobs can be closed")
	ErrDeadlinePassed      = errors.New("the job deadline has passed")
	ErrEmployerNotVerified = errors.New("the employer's company profile is not verified")
	// ErrJobStatusChanged is returned by the repository when a conditional
	// update finds the job no longer in the expected status (a concurrent change).
	ErrJobStatusChanged = errors.New("the job status changed during the request")

	// INTERNAL, with this message instead of the generic one. Returned by the
	// placeholder employer verifier until JT-57 adds the real client.
	ErrEmployerCheckUnavailable = errors.New("employer verification is not available until JT-57")
)
