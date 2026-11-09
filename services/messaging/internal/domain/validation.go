package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnauthenticated   = errors.New("authenticated identity required")
	ErrPermissionDenied  = errors.New("message access denied")
	ErrInvalidPagination = errors.New("invalid pagination")
	ErrInvalidContent    = errors.New("content must contain 1 to 2000 characters after trimming")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NormalizeID(raw string, invalid error) (string, error) {
	if !uuidPattern.MatchString(raw) {
		return "", invalid
	}
	return strings.ToLower(raw), nil
}

func ValidateContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	if strings.ContainsRune(content, 0) || !utf8.ValidString(content) || utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 2000 {
		return "", ErrInvalidContent
	}
	return content, nil
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
