package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits. Noted in the JT-30 PR and used for boundary value tests.
const (
	MaxEmailLength    = 254 // longest valid email address (RFC 5321)
	MaxFullNameLength = 100
	MinPasswordLength = 8
	// bcrypt silently ignores bytes after 72, so longer passwords are rejected.
	MaxPasswordBytes       = 72
	MinAdminPasswordLength = 12
	// Must match the ADMIN_PASSWORD placeholder in deploy/compose/.env.example.
	AdminPasswordPlaceholder = "change-me"
)

var (
	ErrInvalidEmail      = errors.New("email must be a valid address of at most 254 characters")
	ErrInvalidFullName   = errors.New("full name must be 1 to 100 characters")
	ErrInvalidPassword   = errors.New("password must be at least 8 characters and at most 72 bytes")
	ErrRoleNotAllowed    = errors.New("role must be applicant or employer")
	ErrWeakAdminPassword = errors.New("ADMIN_PASSWORD must be set, not the placeholder, and at least 12 characters")
)

// Role matches the JWT "role" claim and the x-user-role metadata value.
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

func (r Role) Valid() bool { return r >= RoleApplicant && r <= RoleAdmin }

// CanSelfRegister is false for admins; they are only seeded at startup.
func (r Role) CanSelfRegister() bool { return r == RoleApplicant || r == RoleEmployer }

// ParseRole converts a claim or metadata value back to a Role.
func ParseRole(s string) (Role, bool) {
	for _, r := range []Role{RoleApplicant, RoleEmployer, RoleAdmin} {
		if r.String() == s {
			return r, true
		}
	}
	return RoleUnspecified, false
}

// User is an account. PasswordHash never leaves the service.
type User struct {
	ID           string
	Email        string
	FullName     string
	Role         Role
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Registration holds validated and normalized sign-up input.
type Registration struct {
	Email    string
	FullName string
	Password string
	Role     Role
}

// NewRegistration validates self-registration input.
func NewRegistration(email, password, fullName string, role Role) (Registration, error) {
	if !role.CanSelfRegister() {
		return Registration{}, ErrRoleNotAllowed
	}
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return Registration{}, err
	}
	name, err := NormalizeFullName(fullName)
	if err != nil {
		return Registration{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Registration{}, err
	}
	return Registration{Email: normalizedEmail, FullName: name, Password: password, Role: role}, nil
}

// NormalizeEmail trims and lowercases so login is case-insensitive.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || utf8.RuneCountInString(email) > MaxEmailLength {
		return "", ErrInvalidEmail
	}
	// Reject display-name forms such as "Name <a@b.bt>".
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func NormalizeFullName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > MaxFullNameLength {
		return "", ErrInvalidFullName
	}
	return name, nil
}

// ValidatePassword does not trim, because spaces can be part of a password.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength || len(password) > MaxPasswordBytes {
		return ErrInvalidPassword
	}
	return nil
}

// ValidateAdminPassword applies the stricter startup rule from AGENTS.md Section 9.
func ValidateAdminPassword(password string) error {
	if password == "" || password == AdminPasswordPlaceholder ||
		utf8.RuneCountInString(password) < MinAdminPasswordLength {
		return ErrWeakAdminPassword
	}
	if len(password) > MaxPasswordBytes {
		return ErrInvalidPassword
	}
	return nil
}
