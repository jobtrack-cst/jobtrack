package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
// Validation errors may be wrapped with the position of the bad entry, for
// example "education[2]: ...", so check them with errors.Is.
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// PERMISSION_DENIED
	ErrApplicantRoleRequired = errors.New("only applicants can manage an applicant profile")

	// INVALID_ARGUMENT
	ErrInvalidSummary       = errors.New("summary must be at most 2000 characters of valid text")
	ErrTooManyEducation     = errors.New("education must have at most 20 entries")
	ErrInvalidInstitution   = errors.New("institution must be 1 to 150 characters on one line")
	ErrInvalidQualification = errors.New("qualification must be 1 to 150 characters on one line")
	ErrInvalidStartYear     = errors.New("start_year must be from 1950 to next year")
	ErrInvalidEndYear       = errors.New("end_year must be 0 (ongoing) or from start_year to 10 years from now")
	ErrTooManySkills        = errors.New("skills must have at most 50 entries")
	ErrInvalidSkill         = errors.New("each skill must be 1 to 50 characters on one line")
	ErrDuplicateSkill       = errors.New("skills must not repeat (compared ignoring case and surrounding spaces)")

	// NOT_FOUND
	ErrProfileNotFound = errors.New("applicant profile not found")

	// ALREADY_EXISTS
	ErrProfileExists = errors.New("applicant profile already exists")
)
