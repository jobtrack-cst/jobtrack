package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("application not found")
	ErrDuplicate = errors.New("application already exists")
	ErrWithdrawn = errors.New("application already withdrawn")
)

type Application struct {
	ID, ApplicantID, JobID, ResumeID, EmployerID string
	Withdrawn                                    bool
	CreatedAt, UpdatedAt                         time.Time
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
