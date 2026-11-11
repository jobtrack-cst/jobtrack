package domain

import (
	"regexp"
	"strings"
)

// Role matches the x-user-role metadata value set by the gateway.
type Role uint8

const (
	RoleUnspecified Role = iota
	RoleApplicant
	RoleEmployer
	RoleAdmin
)

func (r Role) String() string {
	switch r {
	case RoleApplicant:
		return "applicant"
	case RoleEmployer:
		return "employer"
	case RoleAdmin:
		return "admin"
	default:
		return ""
	}
}

// ParseRole accepts only the exact lowercase values from the JWT role claim.
func ParseRole(s string) (Role, bool) {
	for _, r := range []Role{RoleApplicant, RoleEmployer, RoleAdmin} {
		if r.String() == s {
			return r, true
		}
	}
	return RoleUnspecified, false
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeUUID stops malformed IDs before they reach Postgres as INTERNAL
// errors, and lowercases so lookups match the stored uuid text.
func normalizeUUID(raw string) (string, bool) {
	if !uuidPattern.MatchString(raw) {
		return "", false
	}
	return strings.ToLower(raw), true
}

// NormalizeApplicationID validates an application_id from a request or event.
func NormalizeApplicationID(raw string) (string, error) {
	id, ok := normalizeUUID(raw)
	if !ok {
		return "", ErrInvalidApplicationID
	}
	return id, nil
}

// Caller is the identity forwarded by the gateway.
type Caller struct {
	UserID string
	Role   Role
}

// NewCaller rejects identity values the gateway would never set.
func NewCaller(userID, role string) (Caller, error) {
	id, ok := normalizeUUID(userID)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	r, ok := ParseRole(role)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: id, Role: r}, nil
}

// RequireParticipantRole is checked before any lookup: only applicants and
// employers can ever read an application's status, so admins learn nothing.
func (c Caller) RequireParticipantRole() error {
	if c.Role != RoleApplicant && c.Role != RoleEmployer {
		return ErrRoleNotAllowed
	}
	return nil
}
