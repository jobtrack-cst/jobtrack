package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnauthenticated  = errors.New("authenticated identity required")
	ErrPermissionDenied = errors.New("offer access denied")
	ErrInvalidOfferID   = errors.New("offer ID must be a UUID")
	ErrInvalidTerms     = errors.New("terms must contain 1 to 2000 characters after trimming")
	ErrOfferNotFound    = errors.New("offer not found")
	ErrOfferExists      = errors.New("an offer already exists for this application")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NormalizeID(raw string, invalid error) (string, error) {
	if !uuidPattern.MatchString(raw) {
		return "", invalid
	}
	return strings.ToLower(raw), nil
}

func ValidateTerms(raw string) (string, error) {
	terms := strings.TrimSpace(raw)
	if strings.ContainsRune(terms, 0) || !utf8.ValidString(terms) || utf8.RuneCountInString(terms) < 1 || utf8.RuneCountInString(terms) > 2000 {
		return "", ErrInvalidTerms
	}
	return terms, nil
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

func (o Offer) CanView(c Caller) error {
	if _, err := NewCaller(c.UserID, c.Role); err != nil {
		return err
	}
	if (c.Role == "applicant" && strings.EqualFold(c.UserID, o.ApplicantID)) || (c.Role == "employer" && strings.EqualFold(c.UserID, o.EmployerID)) {
		return nil
	}
	return ErrPermissionDenied
}
