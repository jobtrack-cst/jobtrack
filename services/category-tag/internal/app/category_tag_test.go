package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/ports/mocks"
)

const (
	userID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	cat1   = "10000000-0000-4000-8000-000000000001"
	tag1   = "20000000-0000-4000-8000-000000000001"
	tag2   = "20000000-0000-4000-8000-000000000002"
)

var errDB = errors.New("secret SQL detail")

func newService(t *testing.T) (*app.CategoryTag, *mocks.MockReferenceRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockReferenceRepository(ctrl)
	return app.NewCategoryTag(repo), repo
}

func caller(role domain.Role) domain.Caller { return domain.Caller{UserID: userID, Role: role} }

// Every role may read every list; the repository gets the validated page.
func TestCategoryTag_List_AllowsEveryRoleForEveryKind(t *testing.T) {
	for _, role := range []domain.Role{domain.RoleApplicant, domain.RoleEmployer, domain.RoleAdmin} {
		for _, kind := range []domain.Kind{domain.KindCategory, domain.KindTag, domain.KindLocation} {
			t.Run(role.String()+" "+kind.String(), func(t *testing.T) {
				svc, repo := newService(t)
				items := []domain.Item{{ID: cat1, Name: "Design"}}
				repo.EXPECT().List(gomock.Any(), kind, domain.Page{Limit: 20}).Return(items, 1, nil)
				got, err := svc.List(context.Background(), caller(role), kind, 0, "")
				require.NoError(t, err)
				assert.Equal(t, app.ItemPage{Items: items, TotalCount: 1}, got)
			})
		}
	}
}

func TestCategoryTag_List_RejectsBeforeReading(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		kind    domain.Kind
		size    int32
		token   string
		wantErr error
	}{
		{"no caller", domain.Caller{}, domain.KindTag, 0, "", domain.ErrUnauthenticated},
		{"no role", domain.Caller{UserID: userID}, domain.KindTag, 0, "", domain.ErrUnauthenticated},
		// The caller is checked first, so an anonymous caller learns nothing about input rules.
		{"no caller and bad page size", domain.Caller{}, domain.KindTag, -1, "", domain.ErrUnauthenticated},
		{"negative page size", caller(domain.RoleApplicant), domain.KindTag, -1, "", domain.ErrInvalidPageSize},
		{"bad page token", caller(domain.RoleEmployer), domain.KindLocation, 0, "x", domain.ErrInvalidPageToken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newService(t) // no repository call expected
			_, err := svc.List(context.Background(), tt.caller, tt.kind, tt.size, tt.token)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestCategoryTag_List_UnknownKindIsInternal(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.List(context.Background(), caller(domain.RoleAdmin), domain.KindUnspecified, 0, "")
	require.Error(t, err)
}

func TestCategoryTag_List_Pagination(t *testing.T) {
	for _, tt := range []struct {
		name      string
		size      int32
		token     string
		wantPage  domain.Page
		total     int
		wantToken string
	}{
		{"first of two pages", 20, "", domain.Page{Limit: 20}, 30, "20"},
		{"second page", 20, "20", domain.Page{Limit: 20, Offset: 20}, 30, ""},
		{"capped size", 500, "", domain.Page{Limit: 100}, 30, ""},
		{"past the end", 10, "90", domain.Page{Limit: 10, Offset: 90}, 30, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)
			repo.EXPECT().List(gomock.Any(), domain.KindTag, tt.wantPage).Return([]domain.Item{}, tt.total, nil)
			got, err := svc.List(context.Background(), caller(domain.RoleApplicant), domain.KindTag, tt.size, tt.token)
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, got.NextPageToken)
			assert.Equal(t, tt.total, got.TotalCount)
		})
	}
}

func TestCategoryTag_List_RepositoryError(t *testing.T) {
	svc, repo := newService(t)
	repo.EXPECT().List(gomock.Any(), domain.KindCategory, gomock.Any()).Return(nil, 0, errDB)
	_, err := svc.List(context.Background(), caller(domain.RoleAdmin), domain.KindCategory, 0, "")
	assert.ErrorIs(t, err, errDB)
}

func TestCategoryTag_ResolveNames(t *testing.T) {
	catItems := []domain.Item{{ID: cat1, Name: "Software Development"}}
	tagItems := []domain.Item{{ID: tag1, Name: "Go"}}
	for _, tt := range []struct {
		name       string
		categories []string
		tags       []string
		expect     func(repo *mocks.MockReferenceRepository)
		want       app.Resolved
	}{
		{"empty request reads nothing", nil, nil, func(*mocks.MockReferenceRepository) {},
			app.Resolved{Categories: []domain.Item{}, Tags: []domain.Item{}}},
		{"categories only", []string{cat1}, nil, func(r *mocks.MockReferenceRepository) {
			r.EXPECT().Resolve(gomock.Any(), domain.KindCategory, []string{cat1}).Return(catItems, nil)
		}, app.Resolved{Categories: catItems, Tags: []domain.Item{}}},
		{"both lists", []string{cat1}, []string{tag1}, func(r *mocks.MockReferenceRepository) {
			r.EXPECT().Resolve(gomock.Any(), domain.KindCategory, []string{cat1}).Return(catItems, nil)
			r.EXPECT().Resolve(gomock.Any(), domain.KindTag, []string{tag1}).Return(tagItems, nil)
		}, app.Resolved{Categories: catItems, Tags: tagItems}},
		{"duplicates and malformed IDs removed before reading", []string{"bad", "BAD-ID"}, []string{tag1, "x", tag1, tag2}, func(r *mocks.MockReferenceRepository) {
			r.EXPECT().Resolve(gomock.Any(), domain.KindTag, []string{tag1, tag2}).Return(tagItems, nil)
		}, app.Resolved{Categories: []domain.Item{}, Tags: tagItems}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)
			tt.expect(repo)
			got, err := svc.ResolveNames(context.Background(), tt.categories, tt.tags)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCategoryTag_ResolveNames_TooManyIDs(t *testing.T) {
	many := make([]string, 101)
	for i := range many {
		many[i] = tag1
	}
	for _, tt := range []struct {
		name       string
		categories []string
		tags       []string
	}{
		{"101 category IDs", many, nil},
		{"101 tag IDs", []string{cat1}, many},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newService(t) // no repository call expected
			_, err := svc.ResolveNames(context.Background(), tt.categories, tt.tags)
			assert.ErrorIs(t, err, domain.ErrTooManyIDs)
		})
	}
}

func TestCategoryTag_ResolveNames_RepositoryError(t *testing.T) {
	svc, repo := newService(t)
	repo.EXPECT().Resolve(gomock.Any(), domain.KindCategory, []string{cat1}).Return(nil, errDB)
	_, err := svc.ResolveNames(context.Background(), []string{cat1}, []string{tag1})
	assert.ErrorIs(t, err, errDB)
}
