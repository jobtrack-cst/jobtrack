package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// PutPhoto stores or replaces the caller's own photo (JT-103). The role is
// checked before the input, and only the caller's user ID is used as the key,
// so nobody can change another applicant's photo.
func (p *Profiles) PutPhoto(ctx context.Context, caller domain.Caller, contentType string, content []byte) (domain.Profile, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Profile{}, err
	}
	photo, err := domain.NewPhoto(contentType, content)
	if err != nil {
		return domain.Profile{}, err
	}
	return p.repo.PutPhoto(ctx, caller.UserID, photo, p.clock.Now().UTC())
}

// GetPhoto returns the caller's own photo. Only the applicant sees it.
func (p *Profiles) GetPhoto(ctx context.Context, caller domain.Caller) (domain.Photo, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Photo{}, err
	}
	return p.repo.GetPhoto(ctx, caller.UserID)
}
