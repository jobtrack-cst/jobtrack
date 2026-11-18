package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// PutLogo stores or replaces the caller's own logo (JT-102). The role is
// checked before the input, and only the caller's user ID is used as the key,
// so nobody can change another employer's logo. Verification is never touched.
func (p *Profiles) PutLogo(ctx context.Context, caller domain.Caller, contentType string, content []byte) (domain.CompanyProfile, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.CompanyProfile{}, err
	}
	logo, err := domain.NewLogo(contentType, content)
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	updated, err := p.repo.PutLogo(ctx, caller.UserID, logo, p.clock.Now().UTC())
	if err != nil {
		return domain.CompanyProfile{}, err
	}
	return updated.ViewFor(caller), nil
}

// GetLogo returns any employer's logo to any authenticated caller, like Get.
func (p *Profiles) GetLogo(ctx context.Context, _ domain.Caller, employerID string) (domain.Logo, error) {
	id, err := domain.NormalizeEmployerID(employerID)
	if err != nil {
		return domain.Logo{}, err
	}
	return p.repo.GetLogo(ctx, id)
}
