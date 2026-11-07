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

// NormalizeEmployerID stops malformed IDs before they reach Postgres as
// INTERNAL errors, and lowercases so lookups match the stored uuid text.
func NormalizeEmployerID(raw string) (string, error) {
	if !uuidPattern.MatchString(raw) {
		return "", ErrInvalidEmployerID
	}
	return strings.ToLower(raw), nil
}

// Caller is the identity forwarded by the gateway (or by job-posting in Sprint 2).
type Caller struct {
	UserID string
	Role   Role
}

// NewCaller rejects identity values the gateway would never set.
func NewCaller(userID, role string) (Caller, error) {
	id, err := NormalizeEmployerID(userID)
	if err != nil {
		return Caller{}, ErrUnauthenticated
	}
	r, ok := ParseRole(role)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: id, Role: r}, nil
}

// RequireEmployer guards Create, GetMy and Update, which act on the caller's own profile.
func (c Caller) RequireEmployer() error {
	if c.Role != RoleEmployer {
		return ErrEmployerRoleRequired
	}
	return nil
}
