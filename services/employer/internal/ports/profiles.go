package ports

//go:generate go tool mockgen -destination=mocks/profiles_mock.go -package=mocks . ProfileRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// ProfileRepository stores company profiles, one per employer user ID.
// Employer calls no other services, so there are no client ports.
type ProfileRepository interface {
	// Create returns domain.ErrProfileExists if the employer already has a
	// profile. A verification decision stored before the profile existed is
	// copied into the new profile (JT-56).
	Create(ctx context.Context, profile domain.CompanyProfile) (domain.CompanyProfile, error)
	// GetByEmployerID returns domain.ErrProfileNotFound when there is no profile.
	GetByEmployerID(ctx context.Context, employerID string) (domain.CompanyProfile, error)
	// Update writes only the editable fields and updated_at, never the
	// verification columns, so it cannot undo a verification event (JT-56).
	// It returns the stored row, or domain.ErrProfileNotFound.
	Update(ctx context.Context, profile domain.CompanyProfile) (domain.CompanyProfile, error)
}

// Clock makes created_at and updated_at testable.
type Clock interface {
	Now() time.Time
}
