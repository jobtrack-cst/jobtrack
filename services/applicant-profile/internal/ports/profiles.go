package ports

//go:generate go tool mockgen -destination=mocks/profiles_mock.go -package=mocks . ProfileRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// ProfileRepository stores one profile per applicant. Each write stores the
// profile with its education and skills in one transaction.
type ProfileRepository interface {
	// Create stores a new profile, or returns domain.ErrProfileExists.
	Create(ctx context.Context, profile domain.Profile) error
	// Get returns the applicant's profile, or domain.ErrProfileNotFound.
	Get(ctx context.Context, applicantID string) (domain.Profile, error)
	// Replace overwrites the summary, education and skills and sets
	// updated_at, keeping created_at. It returns the stored profile, or
	// domain.ErrProfileNotFound.
	Replace(ctx context.Context, applicantID string, content domain.ProfileContent, updatedAt time.Time) (domain.Profile, error)
}

// Clock makes timestamps and the year limits testable.
type Clock interface {
	Now() time.Time
}
