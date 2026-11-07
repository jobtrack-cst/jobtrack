package domain

import (
	"errors"
	"regexp"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email is already registered")
	// Same error for wrong email and wrong password, so emails cannot be probed.
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthenticated    = errors.New("caller identity is missing")
	ErrInvalidUserID      = errors.New("user_id must be a UUID")
	ErrInvalidRole        = errors.New("role must be applicant, employer or admin")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidateUserID stops malformed IDs before they reach Postgres as INTERNAL errors.
func ValidateUserID(id string) error {
	if !uuidPattern.MatchString(id) {
		return ErrInvalidUserID
	}
	return nil
}
