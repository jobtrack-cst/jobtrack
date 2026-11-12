package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	ctv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/category-tag/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/ports/mocks"
)

const (
	userID = "11111111-1111-4111-8111-111111111111"
	cat1   = "10000000-0000-4000-8000-000000000001"
	tag1   = "20000000-0000-4000-8000-000000000001"
	loc1   = "30000000-0000-4000-8000-000000000014"
)

var errDB = errors.New("pq: secret SQL detail")

// Handlers are tested with the real app layer and a mocked repository.
func newHandlers(t *testing.T) (*adapter.CategoryTagHandler, *adapter.CategoryTagInternalHandler, *mocks.MockReferenceRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockReferenceRepository(ctrl)
	svc := app.NewCategoryTag(repo)
	return adapter.NewCategoryTagHandler(svc), adapter.NewCategoryTagInternalHandler(svc), repo
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

// listCall runs one of the three list RPCs and returns its error.
type listCall struct {
	kind domain.Kind
	call func(h *adapter.CategoryTagHandler, ctx context.Context, p *commonv1.PaginationRequest) error
}

var listCalls = []listCall{
	{domain.KindCategory, func(h *adapter.CategoryTagHandler, ctx context.Context, p *commonv1.PaginationRequest) error {
		_, err := h.ListCategories(ctx, &ctv1.ListCategoriesRequest{Pagination: p})
		return err
	}},
	{domain.KindTag, func(h *adapter.CategoryTagHandler, ctx context.Context, p *commonv1.PaginationRequest) error {
		_, err := h.ListTags(ctx, &ctv1.ListTagsRequest{Pagination: p})
		return err
	}},
	{domain.KindLocation, func(h *adapter.CategoryTagHandler, ctx context.Context, p *commonv1.PaginationRequest) error {
		_, err := h.ListLocations(ctx, &ctv1.ListLocationsRequest{Pagination: p})
		return err
	}},
}

// Every authorization rule, for every list RPC. No repository call is
// expected for the rejected cases.
func TestListRPCs_Authorization(t *testing.T) {
	for _, lc := range listCalls {
		for _, tt := range []struct {
			name string
			ctx  context.Context
			want codes.Code
		}{
			{"applicant", withCaller(userID, "applicant"), codes.OK},
			{"employer", withCaller(userID, "employer"), codes.OK},
			{"admin", withCaller(userID, "admin"), codes.OK},
			{"no metadata", context.Background(), codes.Unauthenticated},
			{"user ID not a UUID", withCaller("user-1", "admin"), codes.Unauthenticated},
			{"unknown role", withCaller(userID, "guest"), codes.Unauthenticated},
			{"role only", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "admin")), codes.Unauthenticated},
		} {
			t.Run(lc.kind.String()+" "+tt.name, func(t *testing.T) {
				h, _, repo := newHandlers(t)
				if tt.want == codes.OK {
					repo.EXPECT().List(gomock.Any(), lc.kind, domain.Page{Limit: 20}).Return([]domain.Item{}, 0, nil)
				}
				wantCode(t, lc.call(h, tt.ctx, nil), tt.want)
			})
		}
	}
}

func TestListRPCs_InputAndErrors(t *testing.T) {
	for _, lc := range listCalls {
		for _, tt := range []struct {
			name   string
			p      *commonv1.PaginationRequest
			expect func(repo *mocks.MockReferenceRepository)
			want   codes.Code
		}{
			{"negative page size", &commonv1.PaginationRequest{PageSize: -1}, func(*mocks.MockReferenceRepository) {}, codes.InvalidArgument},
			{"bad page token", &commonv1.PaginationRequest{PageToken: "next"}, func(*mocks.MockReferenceRepository) {}, codes.InvalidArgument},
			{"repository error hidden", nil, func(r *mocks.MockReferenceRepository) {
				r.EXPECT().List(gomock.Any(), lc.kind, gomock.Any()).Return(nil, 0, errDB)
			}, codes.Internal},
			{"canceled", nil, func(r *mocks.MockReferenceRepository) {
				r.EXPECT().List(gomock.Any(), lc.kind, gomock.Any()).Return(nil, 0, context.Canceled)
			}, codes.Canceled},
		} {
			t.Run(lc.kind.String()+" "+tt.name, func(t *testing.T) {
				h, _, repo := newHandlers(t)
				tt.expect(repo)
				err := lc.call(h, withCaller(userID, "applicant"), tt.p)
				wantCode(t, err, tt.want)
				if err != nil {
					assert.NotContains(t, err.Error(), "secret")
				}
			})
		}
	}
}

func TestListCategories_MapsItemsAndPagination(t *testing.T) {
	h, _, repo := newHandlers(t)
	repo.EXPECT().List(gomock.Any(), domain.KindCategory, domain.Page{Limit: 1}).
		Return([]domain.Item{{ID: cat1, Name: "Software Development"}}, 12, nil)
	resp, err := h.ListCategories(withCaller(userID, "employer"), &ctv1.ListCategoriesRequest{Pagination: &commonv1.PaginationRequest{PageSize: 1}})
	require.NoError(t, err)
	require.Len(t, resp.GetCategories(), 1)
	assert.Equal(t, cat1, resp.GetCategories()[0].GetCategoryId())
	assert.Equal(t, "Software Development", resp.GetCategories()[0].GetName())
	assert.Equal(t, "1", resp.GetPagination().GetNextPageToken())
	assert.Equal(t, int64(12), resp.GetPagination().GetTotalCount())
}

func TestListTags_LastPage(t *testing.T) {
	h, _, repo := newHandlers(t)
	repo.EXPECT().List(gomock.Any(), domain.KindTag, domain.Page{Limit: 20, Offset: 20}).
		Return([]domain.Item{{ID: tag1, Name: "Go"}}, 21, nil)
	resp, err := h.ListTags(withCaller(userID, "admin"), &ctv1.ListTagsRequest{Pagination: &commonv1.PaginationRequest{PageToken: "20"}})
	require.NoError(t, err)
	require.Len(t, resp.GetTags(), 1)
	assert.Equal(t, tag1, resp.GetTags()[0].GetTagId())
	assert.Equal(t, "Go", resp.GetTags()[0].GetName())
	assert.Empty(t, resp.GetPagination().GetNextPageToken())
	assert.Equal(t, int64(21), resp.GetPagination().GetTotalCount())
}

func TestListLocations_MapsItems(t *testing.T) {
	h, _, repo := newHandlers(t)
	repo.EXPECT().List(gomock.Any(), domain.KindLocation, gomock.Any()).
		Return([]domain.Item{{ID: loc1, Name: "Thimphu"}}, 21, nil)
	resp, err := h.ListLocations(withCaller(userID, "applicant"), nil)
	require.NoError(t, err)
	require.Len(t, resp.GetLocations(), 1)
	assert.Equal(t, loc1, resp.GetLocations()[0].GetLocationId())
	assert.Equal(t, "Thimphu", resp.GetLocations()[0].GetName())
}

func TestResolveNames(t *testing.T) {
	_, h, repo := newHandlers(t)
	repo.EXPECT().Resolve(gomock.Any(), domain.KindCategory, []string{cat1}).
		Return([]domain.Item{{ID: cat1, Name: "Software Development"}}, nil)
	repo.EXPECT().Resolve(gomock.Any(), domain.KindTag, []string{tag1}).
		Return([]domain.Item{{ID: tag1, Name: "Go"}}, nil)
	// No user metadata: the internal RPC has no user caller.
	resp, err := h.ResolveNames(context.Background(), &ctv1.ResolveNamesRequest{
		CategoryIds: []string{cat1, "not-a-uuid"},
		TagIds:      []string{strings.ToUpper(tag1), tag1},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetCategories(), 1)
	assert.Equal(t, cat1, resp.GetCategories()[0].GetCategoryId())
	assert.Equal(t, "Software Development", resp.GetCategories()[0].GetName())
	require.Len(t, resp.GetTags(), 1)
	assert.Equal(t, tag1, resp.GetTags()[0].GetTagId())
}

func TestResolveNames_EmptyRequest(t *testing.T) {
	_, h, _ := newHandlers(t) // no repository call expected
	resp, err := h.ResolveNames(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, resp.GetCategories())
	assert.Empty(t, resp.GetTags())
}

func TestResolveNames_Errors(t *testing.T) {
	many := make([]string, 101)
	for i := range many {
		many[i] = tag1
	}
	for _, tt := range []struct {
		name   string
		req    *ctv1.ResolveNamesRequest
		expect func(repo *mocks.MockReferenceRepository)
		want   codes.Code
	}{
		{"too many category IDs", &ctv1.ResolveNamesRequest{CategoryIds: many}, func(*mocks.MockReferenceRepository) {}, codes.InvalidArgument},
		{"too many tag IDs", &ctv1.ResolveNamesRequest{TagIds: many}, func(*mocks.MockReferenceRepository) {}, codes.InvalidArgument},
		{"repository error hidden", &ctv1.ResolveNamesRequest{TagIds: []string{tag1}}, func(r *mocks.MockReferenceRepository) {
			r.EXPECT().Resolve(gomock.Any(), domain.KindTag, gomock.Any()).Return(nil, errDB)
		}, codes.Internal},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, h, repo := newHandlers(t)
			tt.expect(repo)
			_, err := h.ResolveNames(context.Background(), tt.req)
			wantCode(t, err, tt.want)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}
