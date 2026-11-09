package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const owner = "11111111-1111-1111-1111-111111111111"
const category = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func TestAlerts_AllOperationsRejectUnauthorizedCallers(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    domain.Caller
		err  error
	}{
		{"missing", domain.Caller{}, domain.ErrUnauthenticated}, {"invalid ID", domain.Caller{UserID: "bad", Role: "applicant"}, domain.ErrUnauthenticated}, {"unknown role", domain.Caller{UserID: owner, Role: "unknown"}, domain.ErrUnauthenticated}, {"employer", domain.Caller{UserID: owner, Role: "employer"}, domain.ErrPermissionDenied}, {"admin", domain.Caller{UserID: owner, Role: "admin"}, domain.ErrPermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			s := app.NewAlerts(mocks.NewMockAlertRepository(ctrl), mocks.NewMockNameResolver(ctrl), nil)
			ctx := context.Background()
			_, err := s.Create(ctx, tc.c, nil, nil, "Thimphu")
			require.ErrorIs(t, err, tc.err)
			_, _, _, err = s.List(ctx, tc.c, 0, "")
			require.ErrorIs(t, err, tc.err)
			require.ErrorIs(t, s.Delete(ctx, tc.c, category), tc.err)
		})
	}
}
func TestAlerts_CreateResolvesNormalizedFiltersAndPersistsNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAlertRepository(ctrl)
	resolver := mocks.NewMockNameResolver(ctrl)
	clock := mocks.NewMockClock(ctrl)
	ctx := context.Background()
	now := time.Now()
	names := domain.Names{Categories: map[string]string{category: "Engineering"}, Tags: map[string]string{owner: "Go"}}
	resolver.EXPECT().ResolveNames(ctx, []string{category}, []string{owner}).Return(names, nil)
	clock.EXPECT().Now().Return(now)
	repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, a domain.Alert) (domain.Alert, error) {
		require.Equal(t, owner, a.ApplicantID)
		require.Equal(t, names, a.Names)
		require.Equal(t, now.UTC(), a.CreatedAt)
		require.Equal(t, a.CreatedAt, a.UpdatedAt)
		_, err := domain.NormalizeID(a.ID, domain.ErrInvalid)
		require.NoError(t, err)
		return a, nil
	})
	a, err := app.NewAlerts(repo, resolver, clock).Create(ctx, domain.Caller{UserID: owner, Role: "applicant"}, []string{category, category}, []string{owner}, "Thimphu")
	require.NoError(t, err)
	require.Equal(t, []string{category}, a.CategoryIDs)
}
func TestAlerts_CreateFailures(t *testing.T) {
	dependencyErr := errors.New("resolver failed")
	for _, tc := range []struct {
		name                      string
		names                     domain.Names
		resolveErr, repoErr, want error
	}{
		{"missing category", domain.Names{}, nil, nil, domain.ErrInvalid},
		{"dependency", domain.Names{}, dependencyErr, nil, dependencyErr},
		{"duplicate", domain.Names{Categories: map[string]string{category: "name"}}, nil, domain.ErrExists, domain.ErrExists},
		{"limit", domain.Names{Categories: map[string]string{category: "name"}}, nil, domain.ErrLimit, domain.ErrLimit},
		{"database", domain.Names{Categories: map[string]string{category: "name"}}, nil, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockAlertRepository(ctrl)
			resolver := mocks.NewMockNameResolver(ctrl)
			resolver.EXPECT().ResolveNames(gomock.Any(), []string{category}, []string{}).Return(tc.names, tc.resolveErr)
			if tc.repoErr != nil {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Alert{}, tc.repoErr)
			}
			_, err := app.NewAlerts(repo, resolver, nil).Create(context.Background(), domain.Caller{UserID: owner, Role: "applicant"}, []string{category}, nil, "")
			require.ErrorIs(t, err, tc.want)
		})
	}
}
func TestAlerts_LocationOnlyAndStandaloneResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAlertRepository(ctrl)
	s := app.NewAlerts(repo, nil, nil)
	c := domain.Caller{UserID: owner, Role: "applicant"}
	ctx := context.Background()
	repo.EXPECT().Create(ctx, gomock.Any()).Return(domain.Alert{Location: "Thimphu"}, nil)
	_, err := s.Create(ctx, c, nil, nil, "Thimphu")
	require.NoError(t, err)
	_, err = s.Create(ctx, c, []string{category}, nil, "")
	require.ErrorIs(t, err, domain.ErrResolverUnavailable)
	_, err = s.Create(ctx, c, nil, nil, "")
	require.ErrorIs(t, err, domain.ErrInvalid)
}
func TestAlerts_ListAndDeleteScopeAndErrors(t *testing.T) {
	for _, want := range []error{nil, domain.ErrPermissionDenied, domain.ErrNotFound, context.Canceled} {
		t.Run(errorName(want), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockAlertRepository(ctrl)
			s := app.NewAlerts(repo, nil, nil)
			c := domain.Caller{UserID: owner, Role: "applicant"}
			ctx := context.Background()
			repo.EXPECT().List(ctx, owner, domain.Page{Size: 1, Offset: 2}).Return([]domain.Alert{{ApplicantID: owner}}, int64(4), want)
			items, total, p, err := s.List(ctx, c, 1, "2")
			require.ErrorIs(t, err, want)
			require.Len(t, items, 1)
			require.EqualValues(t, 4, total)
			require.EqualValues(t, 2, p.Offset)
			repo.EXPECT().Delete(ctx, category, owner).Return(want)
			require.ErrorIs(t, s.Delete(ctx, c, category), want)
			_, _, _, err = s.List(ctx, c, -1, "")
			require.ErrorIs(t, err, domain.ErrPagination)
			require.ErrorIs(t, s.Delete(ctx, c, "bad"), domain.ErrInvalid)
		})
	}
}
func errorName(err error) string {
	if err == nil {
		return "success"
	}
	return err.Error()
}
