package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

// Profiles holds the company profile use cases. The gRPC adapter builds the
// Caller from pkg/authctx and translates domain errors to gRPC codes.
type Profiles struct {
	repo  ports.ProfileRepository
	clock ports.Clock
}

func NewProfiles(repo ports.ProfileRepository, clock ports.Clock) *Profiles {
	return &Profiles{repo: repo, clock: clock}
}

// ProfileInput is the editable part of create and update requests.
type ProfileInput struct {
	CompanyName string
	Description string
	Website     string
	Location    string
}

func (in ProfileInput) fields() (domain.ProfileFields, error) {
	return domain.NewProfileFields(in.CompanyName, in.Description, in.Website, in.Location)
}

// Create stores the caller's profile, keyed by the caller's own user ID.
// The role is checked before the input, so non-employers learn nothing else.
func (p *Profiles) Create(ctx context.Context, caller domain.Caller, in ProfileInput) (domain.CompanyProfile, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.CompanyProfile{}, err
	}
	f, err := in.fields()
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	created, err := p.repo.Create(ctx, domain.NewCompanyProfile(caller.UserID, f, p.clock.Now()))
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	return created.ViewFor(caller), nil
}

// GetMine returns the caller's own profile.
func (p *Profiles) GetMine(ctx context.Context, caller domain.Caller) (domain.CompanyProfile, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.CompanyProfile{}, err
	}
	profile, err := p.repo.GetByEmployerID(ctx, caller.UserID)
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	return profile.ViewFor(caller), nil
}

// Update replaces the editable fields of the caller's own profile. Only the
// caller's user ID is ever used as the key, so nobody can edit another
// employer's profile.
func (p *Profiles) Update(ctx context.Context, caller domain.Caller, in ProfileInput) (domain.CompanyProfile, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.CompanyProfile{}, err
	}
	f, err := in.fields()
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	existing, err := p.repo.GetByEmployerID(ctx, caller.UserID)
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	updated, err := p.repo.Update(ctx, existing.WithFields(f, p.clock.Now()))
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	return updated.ViewFor(caller), nil
}

// Get returns any employer's profile to any authenticated caller. job-posting
// uses it in Sprint 2 with the user's identity forwarded.
func (p *Profiles) Get(ctx context.Context, caller domain.Caller, employerID string) (domain.CompanyProfile, error) {
	id, err := domain.NormalizeEmployerID(employerID)
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	profile, err := p.repo.GetByEmployerID(ctx, id)
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	return profile.ViewFor(caller), nil
}
