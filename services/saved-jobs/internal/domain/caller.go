package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrUnauthenticated = errors.New("caller identity is missing or invalid")
	ErrApplicantOnly   = errors.New("only applicants can save jobs")
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsUUID(s string) bool { return uuidRE.MatchString(s) }

type Caller struct {
	UserID string
	Role   string
}

func NewCaller(userID, role string) (Caller, error) {
	uid := strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	if !IsUUID(uid) || role == "" {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{UserID: uid, Role: role}, nil
}

func (c Caller) RequireApplicant() error {
	if c.Role != "applicant" {
		return ErrApplicantOnly
	}
	return nil
}
