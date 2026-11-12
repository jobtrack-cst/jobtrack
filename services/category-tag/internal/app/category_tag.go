package app

import (
	"context"
	"fmt"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/ports"
)

// CategoryTag holds the read-only reference list use cases.
type CategoryTag struct {
	repo ports.ReferenceRepository
}

func NewCategoryTag(repo ports.ReferenceRepository) *CategoryTag {
	return &CategoryTag{repo: repo}
}

// ItemPage is one page of a reference list.
type ItemPage struct {
	Items         []domain.Item
	NextPageToken string
	TotalCount    int
}

// List serves ListCategories, ListTags and ListLocations. The caller is
// checked before the pagination input.
func (c *CategoryTag) List(ctx context.Context, caller domain.Caller, kind domain.Kind, pageSize int32, pageToken string) (ItemPage, error) {
	if err := caller.RequireAuthenticated(); err != nil {
		return ItemPage{}, err
	}
	if kind.String() == "" {
		return ItemPage{}, fmt.Errorf("unknown reference kind %d", kind)
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return ItemPage{}, err
	}
	items, total, err := c.repo.List(ctx, kind, page)
	if err != nil {
		return ItemPage{}, fmt.Errorf("list %s items: %w", kind, err)
	}
	return ItemPage{Items: items, NextPageToken: page.NextPageToken(total), TotalCount: total}, nil
}

// Resolved is the ResolveNames result.
type Resolved struct {
	Categories []domain.Item
	Tags       []domain.Item
}

// ResolveNames has no user caller. The internal server interceptor has
// already checked INTERNAL_TOKEN and that the caller is job-alert. Both lists
// are checked before any database read.
func (c *CategoryTag) ResolveNames(ctx context.Context, categoryIDs, tagIDs []string) (Resolved, error) {
	categories, err := domain.ResolveIDs(categoryIDs)
	if err != nil {
		return Resolved{}, err
	}
	tags, err := domain.ResolveIDs(tagIDs)
	if err != nil {
		return Resolved{}, err
	}
	out := Resolved{Categories: []domain.Item{}, Tags: []domain.Item{}}
	if len(categories) > 0 {
		if out.Categories, err = c.repo.Resolve(ctx, domain.KindCategory, categories); err != nil {
			return Resolved{}, fmt.Errorf("resolve categories: %w", err)
		}
	}
	if len(tags) > 0 {
		if out.Tags, err = c.repo.Resolve(ctx, domain.KindTag, tags); err != nil {
			return Resolved{}, fmt.Errorf("resolve tags: %w", err)
		}
	}
	return out, nil
}
