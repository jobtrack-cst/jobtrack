package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("application not found")
	ErrDuplicate = errors.New("application already exists")
	ErrWithdrawn = errors.New("application already withdrawn")
)

type Application struct {
	ID, ApplicantID, JobID, ResumeID, EmployerID string
	// ApplicantName is copied from auth once at submission (JT-114); it is
	// empty for applications submitted before then.
	ApplicantName        string
	Withdrawn            bool
	CreatedAt, UpdatedAt time.Time
}

// MaxApplicantNameLength matches the database CHECK. Auth allows 100
// characters, so the column leaves room without truncating a stored name.
const MaxApplicantNameLength = 200

// ValidApplicantName accepts a name from auth for a new application: valid
// UTF-8 without NUL (PostgreSQL text cannot hold it), not blank, and 1 to 200
// characters. The name is stored exactly as auth returns it.
func ValidApplicantName(name string) bool {
	if !utf8.ValidString(name) || strings.ContainsRune(name, 0) || strings.TrimSpace(name) == "" {
		return false
	}
	return utf8.RuneCountInString(name) <= MaxApplicantNameLength
}

type Job struct {
	ID, EmployerID string
	Published      bool
	Deadline       time.Time
}

type Resume struct {
	ID, ApplicantID string
	Validated       bool
}

// ValidID accepts the canonical UUID representation used by persisted identifiers.
func ValidID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (j Job) Eligible(now time.Time) bool {
	return j.Published && now.Before(j.Deadline)
}
