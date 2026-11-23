package ports

//go:generate go tool mockgen -destination=mocks/application_participants_mock.go -package=mocks . ApplicationParticipantRepository

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// ApplicationParticipantRepository stores the application.submitted
// projection (JT-114 Part C).
type ApplicationParticipantRepository interface {
	// Record claims eventID in processed_events and stores the row in one
	// transaction. A repeated eventID changes nothing; the same row under a new
	// eventID is accepted. Different participants for a stored application
	// return an events.Permanent error and keep no claim.
	Record(ctx context.Context, eventID string, row domain.ApplicationParticipant) error
	// GetByApplicationID returns the stored row or domain.ErrApplicationNotFound.
	// The ID must already be normalized.
	GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipant, error)
}
