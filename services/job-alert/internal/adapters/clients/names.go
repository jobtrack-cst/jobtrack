package clients

import (
	"context"
	categoryv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
	"strings"
)

type Names struct {
	client categoryv1.CategoryTagInternalServiceClient
}

var _ ports.NameResolver = (*Names)(nil)

func NewNames(client categoryv1.CategoryTagInternalServiceClient) *Names { return &Names{client} }
func (n *Names) ResolveNames(ctx context.Context, categories, tags []string) (domain.Names, error) {
	response, err := n.client.ResolveNames(ctx, &categoryv1.ResolveNamesRequest{CategoryIds: categories, TagIds: tags})
	if err != nil {
		if ctx.Err() != nil {
			return domain.Names{}, ctx.Err()
		}
		return domain.Names{}, domain.ErrResolverUnavailable
	}
	names := domain.Names{Categories: map[string]string{}, Tags: map[string]string{}}
	for _, c := range response.GetCategories() {
		names.Categories[strings.ToLower(c.GetCategoryId())] = c.GetName()
	}
	for _, t := range response.GetTags() {
		names.Tags[strings.ToLower(t.GetTagId())] = t.GetName()
	}
	return names, nil
}
