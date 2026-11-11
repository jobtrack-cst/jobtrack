package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrApplicantRoleRequired = errors.New("only applicants can manage resumes")
	ErrNotResumeOwner        = errors.New("only the applicant who uploaded this resume can access it")

	// INVALID_ARGUMENT
	ErrInvalidResumeID    = errors.New("resume_id must be a UUID")
	ErrInvalidFileName    = errors.New(`file_name must be 1 to 255 characters without "/" or "\"`)
	ErrInvalidContentType = errors.New("content_type must be application/pdf")
	ErrInvalidContentSize = errors.New("content must be 1 to 2000000 bytes")
	ErrInvalidPDF         = errors.New("content must be a PDF file")
	ErrInvalidPageSize    = errors.New("page_size must not be negative")
	ErrInvalidPageToken   = errors.New("page_token must be an offset as a decimal string")

	// NOT_FOUND
	ErrResumeNotFound = errors.New("resume not found")
)
