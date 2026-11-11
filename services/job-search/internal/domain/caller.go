package domain

import (
	"errors"
	"regexp"
	"strings"
)

// ErrUnauthenticated is returned when caller identity is missing or invalid.
var ErrUnauthenticated = errors.New("caller identity is missing or invalid")

// ErrInvalidCategoryID etc. live in search.go; keep this file focused on caller.

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether s is a canonical 8-4-4-4-12 hex UUID.
func IsUUID(s string) bool { return uuidRE.MatchString(s) }

// Caller is the authenticated user from gateway metadata.
type Caller struct {
	UserID string
	Role   string
}

// NewCaller validates the forwarded identity. Unknown/empty role or a
// non-UUID user ID is UNAUTHENTICATED (AGENTS.md Section 9).
func NewCaller(userID, role string) (Caller, error) {
	uid := strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	if !IsUUID(uid) || role == "" || !validRole(role) {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: uid, Role: role}, nil
}

func validRole(role string) bool {
	switch role {
	case "applicant", "employer", "admin":
		return true
	}
	return false
}