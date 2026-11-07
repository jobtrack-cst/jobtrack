package ports

//go:generate go tool mockgen -destination=mocks/participants_mock.go -package=mocks . ParticipantRepository

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
)

// ParticipantRepository reads the local trusted application projection.
type ParticipantRepository interface {
	// GetByApplicationID returns domain.ErrParticipantsNotFound for an unknown or
	// unprocessed application, domain.ErrInvalidApplicationID for malformed input,
	// and a different error for infrastructure failures. IDs use UUID strings.
	GetByApplicationID(context.Context, string) (domain.ApplicationParticipants, error)
}
