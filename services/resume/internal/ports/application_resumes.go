package ports

//go:generate go tool mockgen -destination=mocks/application_resumes_mock.go -package=mocks . ApplicationResumeRepository

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

// ApplicationResumeRepository stores the application.submitted projection
// (JT-114 Part B).
type ApplicationResumeRepository interface {
	// Record claims eventID in processed_events and stores the row in one
	// transaction. A repeated eventID changes nothing; the same row under a new
	// eventID is accepted. Different participants for a stored application
	// return an events.Permanent error and keep no claim.
	Record(ctx context.Context, eventID string, row domain.ApplicationResume) error
	// GetByApplicationID returns the stored row or domain.ErrApplicationNotFound.
	// The ID must already be normalized.
	GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationResume, error)
}
