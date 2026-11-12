package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidJobID = errors.New("job_id must be a valid UUID")
	ErrAlreadySaved = errors.New("job is already saved by this applicant")
	ErrNotSaved     = errors.New("job is not saved by this applicant")
	ErrJobClosed    = errors.New("cannot save a job that is closed")
)

type SavedJob struct {
	JobID       string
	ApplicantID string
	SavedAt     time.Time
}

type Page struct {
	Jobs  []SavedJob
	Total int
}
