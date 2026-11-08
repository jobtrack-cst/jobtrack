package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
)

type Alerts struct {
	repo     ports.AlertRepository
	resolver ports.NameResolver
	clock    ports.Clock
}

func NewAlerts(repo ports.AlertRepository, resolver ports.NameResolver, clock ports.Clock) *Alerts {
	return &Alerts{repo, resolver, clock}
}
func (s *Alerts) Create(ctx context.Context, c domain.Caller, categories, tags []string, location string) (domain.Alert, error) {
	if err := c.RequireRole("applicant"); err != nil {
		return domain.Alert{}, err
	}
	categories, tags, err := domain.NormalizeFilters(categories, tags, location)
	if err != nil {
		return domain.Alert{}, err
	}
	names := domain.Names{Categories: map[string]string{}, Tags: map[string]string{}}
	if len(categories)+len(tags) > 0 {
		if s.resolver == nil {
			return domain.Alert{}, domain.ErrResolverUnavailable
		}
		names, err = s.resolver.ResolveNames(ctx, categories, tags)
		if err != nil {
			return domain.Alert{}, err
		}
		if err = names.Validate(categories, tags); err != nil {
			return domain.Alert{}, err
		}
	}
	now := time.Now().UTC()
	if s.clock != nil {
		now = s.clock.Now().UTC()
	}
	return s.repo.Create(ctx, domain.Alert{ID: uuid.NewString(), ApplicantID: strings.ToLower(c.UserID), CategoryIDs: categories, TagIDs: tags, Location: location, Names: names, CreatedAt: now, UpdatedAt: now})
}
func (s *Alerts) List(ctx context.Context, c domain.Caller, size int32, token string) ([]domain.Alert, int64, domain.Page, error) {
	if err := c.RequireRole("applicant"); err != nil {
		return nil, 0, domain.Page{}, err
	}
	page, err := domain.ParsePage(size, token)
	if err != nil {
		return nil, 0, page, err
	}
	alerts, total, err := s.repo.List(ctx, strings.ToLower(c.UserID), page)
	return alerts, total, page, err
}
func (s *Alerts) Delete(ctx context.Context, c domain.Caller, id string) error {
	if err := c.RequireRole("applicant"); err != nil {
		return err
	}
	id, err := domain.NormalizeID(id, domain.ErrInvalid)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, strings.ToLower(c.UserID))
}
