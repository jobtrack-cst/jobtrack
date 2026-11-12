package domain

import "errors"

// Each error maps to one gRPC code in the gRPC adapter (AGENTS.md Section 9).
var (
	// UNAUTHENTICATED
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")

	// INVALID_ARGUMENT
	ErrInvalidPageSize  = errors.New("page_size must not be negative")
	ErrInvalidPageToken = errors.New("page_token must be an offset as a decimal string")
	ErrTooManyIDs       = errors.New("category_ids and tag_ids must each have at most 100 IDs")
)
