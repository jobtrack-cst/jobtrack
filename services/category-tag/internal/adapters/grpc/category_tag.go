package grpc

import (
	"context"
	"errors"

	categorytagv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

// CategoryTagHandler serves the public CategoryTagService through the
// gateway. Any authenticated role may read the lists.
type CategoryTagHandler struct {
	categorytagv1.UnimplementedCategoryTagServiceServer
	svc *app.CategoryTag
}

var _ categorytagv1.CategoryTagServiceServer = (*CategoryTagHandler)(nil)

func NewCategoryTagHandler(svc *app.CategoryTag) *CategoryTagHandler {
	return &CategoryTagHandler{svc: svc}
}

// The generated getters are nil-safe, so a nil request or pagination means
// the default first page.
func (h *CategoryTagHandler) ListCategories(ctx context.Context, req *categorytagv1.ListCategoriesRequest) (*categorytagv1.ListCategoriesResponse, error) {
	page, err := h.list(ctx, domain.KindCategory, req.GetPagination())
	if err != nil {
		return nil, err
	}
	out := make([]*categorytagv1.Category, len(page.Items))
	for i, item := range page.Items {
		out[i] = &categorytagv1.Category{CategoryId: item.ID, Name: item.Name}
	}
	return &categorytagv1.ListCategoriesResponse{Categories: out, Pagination: pageToProto(page)}, nil
}

func (h *CategoryTagHandler) ListTags(ctx context.Context, req *categorytagv1.ListTagsRequest) (*categorytagv1.ListTagsResponse, error) {
	page, err := h.list(ctx, domain.KindTag, req.GetPagination())
	if err != nil {
		return nil, err
	}
	return &categorytagv1.ListTagsResponse{Tags: tagsToProto(page.Items), Pagination: pageToProto(page)}, nil
}

func (h *CategoryTagHandler) ListLocations(ctx context.Context, req *categorytagv1.ListLocationsRequest) (*categorytagv1.ListLocationsResponse, error) {
	page, err := h.list(ctx, domain.KindLocation, req.GetPagination())
	if err != nil {
		return nil, err
	}
	out := make([]*categorytagv1.Location, len(page.Items))
	for i, item := range page.Items {
		out[i] = &categorytagv1.Location{LocationId: item.ID, Name: item.Name}
	}
	return &categorytagv1.ListLocationsResponse{Locations: out, Pagination: pageToProto(page)}, nil
}

func (h *CategoryTagHandler) list(ctx context.Context, kind domain.Kind, p *commonv1.PaginationRequest) (app.ItemPage, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return app.ItemPage{}, toStatus(err)
	}
	page, err := h.svc.List(ctx, caller, kind, p.GetPageSize(), p.GetPageToken())
	if err != nil {
		return app.ItemPage{}, toStatus(err)
	}
	return page, nil
}

// CategoryTagInternalHandler serves CategoryTagInternalService. The gateway
// never registers it, and the internal server interceptor (pkg/authctx) has
// already checked INTERNAL_TOKEN and that the caller is job-alert, so there
// is no user role to check here.
type CategoryTagInternalHandler struct {
	categorytagv1.UnimplementedCategoryTagInternalServiceServer
	svc *app.CategoryTag
}

var _ categorytagv1.CategoryTagInternalServiceServer = (*CategoryTagInternalHandler)(nil)

func NewCategoryTagInternalHandler(svc *app.CategoryTag) *CategoryTagInternalHandler {
	return &CategoryTagInternalHandler{svc: svc}
}

func (h *CategoryTagInternalHandler) ResolveNames(ctx context.Context, req *categorytagv1.ResolveNamesRequest) (*categorytagv1.ResolveNamesResponse, error) {
	resolved, err := h.svc.ResolveNames(ctx, req.GetCategoryIds(), req.GetTagIds())
	if err != nil {
		return nil, toStatus(err)
	}
	categories := make([]*categorytagv1.Category, len(resolved.Categories))
	for i, item := range resolved.Categories {
		categories[i] = &categorytagv1.Category{CategoryId: item.ID, Name: item.Name}
	}
	return &categorytagv1.ResolveNamesResponse{Categories: categories, Tags: tagsToProto(resolved.Tags)}, nil
}

func tagsToProto(items []domain.Item) []*categorytagv1.Tag {
	out := make([]*categorytagv1.Tag, len(items))
	for i, item := range items {
		out[i] = &categorytagv1.Tag{TagId: item.ID, Name: item.Name}
	}
	return out
}

func pageToProto(p app.ItemPage) *commonv1.PaginationResponse {
	return &commonv1.PaginationResponse{NextPageToken: p.NextPageToken, TotalCount: int64(p.TotalCount)}
}

// callerFrom trusts x-user-id and x-user-role only because the gateway sets
// them from a verified JWT and strips client-sent copies (JT-31).
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidPageSize), errors.Is(err, domain.ErrInvalidPageToken),
		errors.Is(err, domain.ErrTooManyIDs):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		// Never return repository details to the client.
		return status.Error(codes.Internal, "internal error")
	}
}
