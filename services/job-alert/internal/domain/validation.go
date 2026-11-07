package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrUnauthenticated     = errors.New("authenticated identity required")
	ErrPermissionDenied    = errors.New("subscription access denied")
	ErrInvalid             = errors.New("invalid subscription filters or ID")
	ErrPagination          = errors.New("invalid pagination")
	ErrNotFound            = errors.New("subscription not found")
	ErrExists              = errors.New("subscription already exists")
	ErrLimit               = errors.New("maximum 10 subscriptions per applicant")
	ErrResolverUnavailable = errors.New("category/tag resolver unavailable in Sprint 1")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NormalizeID(raw string, invalid error) (string, error) {
	if !uuidPattern.MatchString(raw) {
		return "", invalid
	}
	return strings.ToLower(raw), nil
}

// Caller carries the identity established by the gateway.
type Caller struct{ UserID, Role string }

func NewCaller(userID, role string) (Caller, error) {
	id, err := NormalizeID(userID, ErrUnauthenticated)
	if err != nil || (role != "applicant" && role != "employer" && role != "admin") {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: id, Role: role}, nil
}

func (c Caller) RequireRole(role string) error {
	if _, err := NewCaller(c.UserID, c.Role); err != nil {
		return err
	}
	if c.Role != role {
		return ErrPermissionDenied
	}
	return nil
}
