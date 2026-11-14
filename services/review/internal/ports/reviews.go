package ports

//go:generate go tool mockgen -destination=mocks/reviews_mock.go -package=mocks . ParticipantRepository,ReviewRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

// ParticipantRepository reads the local trusted projection of
// application.submitted, filled by the JT-79 consumer through
// postgres.Participants.RecordApplication.
type ParticipantRepository interface {
	// GetByApplicationID returns domain.ErrApplicationNotFound for an unknown
	// or not yet processed application.
	GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipants, error)
}

// ReviewRepository stores at most one completed review per application.
type ReviewRepository interface {
	// Create stores the review and its review.completed outbox event in one
	// transaction, or returns domain.ErrReviewExists if the application
	// already has a review (then nothing is stored).
	Create(ctx context.Context, review domain.Review) error
	// GetByApplicationID returns the review, or domain.ErrReviewNotFound.
	GetByApplicationID(ctx context.Context, applicationID string) (domain.Review, error)
}

// Clock makes created_at and updated_at testable.
type Clock interface {
	Now() time.Time
}
