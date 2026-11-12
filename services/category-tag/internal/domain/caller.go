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

// IsUUID reports whether s has the UUID text form Postgres accepts here.
func IsUUID(s string) bool { return uuidPattern.MatchString(s) }

// Caller is the identity forwarded by the gateway through pkg/authctx.
type Caller struct {
	UserID string
	Role   Role
}

// NewCaller rejects identity values the gateway would never set.
func NewCaller(userID, role string) (Caller, error) {
	if !IsUUID(userID) {
		return Caller{}, ErrUnauthenticated
	}
	r, ok := ParseRole(role)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: strings.ToLower(userID), Role: r}, nil
}

// RequireAuthenticated guards the public list RPCs. Every role may read the
// reference lists (approved for JT-39); there is no ownership to check.
func (c Caller) RequireAuthenticated() error {
	if c.Role == RoleUnspecified || !IsUUID(c.UserID) {
		return ErrUnauthenticated
	}
	return nil
}
