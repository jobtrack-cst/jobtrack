package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports"
)

// Profiles holds the applicant profile use cases. The gRPC adapter builds the
// Caller from pkg/authctx and translates domain errors to gRPC codes. Every
// use case works only on the caller's own profile: the applicant ID always
// comes from the caller, never from the request.
type Profiles struct {
	repo  ports.ProfileRepository
	clock ports.Clock
}

func NewProfiles(repo ports.ProfileRepository, clock ports.Clock) *Profiles {
	return &Profiles{repo: repo, clock: clock}
}

// Create stores the caller's first profile. The role is checked before the
// input, so other roles learn nothing else.
func (p *Profiles) Create(ctx context.Context, caller domain.Caller, in domain.ProfileInput) (domain.Profile, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Profile{}, err
	}
	now := p.clock.Now().UTC()
	content, err := domain.NewProfileContent(in, now)
	if err != nil {
		return domain.Profile{}, err
	}
	profile := domain.Profile{ApplicantID: caller.UserID, ProfileContent: content, CreatedAt: now, UpdatedAt: now}
	if err := p.repo.Create(ctx, profile); err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

// Get returns the caller's own profile.
func (p *Profiles) Get(ctx context.Context, caller domain.Caller) (domain.Profile, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Profile{}, err
	}
	return p.repo.Get(ctx, caller.UserID)
}

// Update replaces the summary, education and skills of the caller's own
// profile; created_at is kept.
func (p *Profiles) Update(ctx context.Context, caller domain.Caller, in domain.ProfileInput) (domain.Profile, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Profile{}, err
	}
	now := p.clock.Now().UTC()
	content, err := domain.NewProfileContent(in, now)
	if err != nil {
		return domain.Profile{}, err
	}
	return p.repo.Replace(ctx, caller.UserID, content, now)
}
