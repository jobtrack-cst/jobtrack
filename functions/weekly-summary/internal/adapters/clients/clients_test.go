package clients_test

import (
	"context"
	"errors"
	"testing"
	"time"

	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
)

type fakeAnalytics struct {
	got  *analyticsv1.GetInternalSummaryRequest
	resp *analyticsv1.GetInternalSummaryResponse
	err  error
}

func (f *fakeAnalytics) GetInternalSummary(_ context.Context, in *analyticsv1.GetInternalSummaryRequest, _ ...grpc.CallOption) (*analyticsv1.GetInternalSummaryResponse, error) {
	f.got = in
	return f.resp, f.err
}

type fakeAuth struct {
	got  *authv1.ListUsersByRoleRequest
	resp *authv1.ListUsersByRoleResponse
	err  error
}

func (f *fakeAuth) ListUsersByRole(_ context.Context, in *authv1.ListUsersByRoleRequest, _ ...grpc.CallOption) (*authv1.ListUsersByRoleResponse, error) {
	f.got = in
	return f.resp, f.err
}

var (
	start = time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC)
	end   = start.Add(domain.Window)
)

func TestAnalytics_Summary_MapsEveryCount(t *testing.T) {
	api := &fakeAnalytics{resp: &analyticsv1.GetInternalSummaryResponse{Summary: &analyticsv1.Summary{
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(end),
		JobsPublished: 1, JobsClosed: 2, ApplicationsSubmitted: 3, ApplicationsWithdrawn: 4,
		OffersMade: 5, OffersAccepted: 6, OffersDeclined: 7,
		StatusChanges: []*analyticsv1.StatusCount{{Status: "SHORTLISTED", Count: 8}, {Status: "UNDER_REVIEW", Count: 9}},
	}}}
	got, err := clients.Analytics{API: api}.Summary(context.Background(), start, end)
	require.NoError(t, err)
	assert.Equal(t, start, api.got.StartTime.AsTime())
	assert.Equal(t, end, api.got.EndTime.AsTime())
	assert.Equal(t, domain.Summary{
		Start: start, End: end,
		JobsPublished: 1, JobsClosed: 2, ApplicationsSubmitted: 3, ApplicationsWithdrawn: 4,
		OffersMade: 5, OffersAccepted: 6, OffersDeclined: 7,
		StatusChanges: []domain.StatusCount{{Status: "SHORTLISTED", Count: 8}, {Status: "UNDER_REVIEW", Count: 9}},
	}, got)
}

func TestAnalytics_Summary_Errors(t *testing.T) {
	rpcErr := errors.New("unavailable")
	for _, tt := range []struct {
		name string
		api  *fakeAnalytics
	}{
		{"RPC error", &fakeAnalytics{err: rpcErr}},
		{"empty response", &fakeAnalytics{resp: &analyticsv1.GetInternalSummaryResponse{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := clients.Analytics{API: tt.api}.Summary(context.Background(), start, end)
			require.Error(t, err)
			if tt.api.err != nil {
				assert.ErrorIs(t, err, rpcErr)
			}
		})
	}
}

func TestAuth_ListAdmins_AsksForAdminsAndKeepsOnlyAdmins(t *testing.T) {
	api := &fakeAuth{resp: &authv1.ListUsersByRoleResponse{
		Users: []*authv1.User{
			{Email: "a@cst.edu.bt", Role: authv1.Role_ROLE_ADMIN},
			{Email: "student@cst.edu.bt", Role: authv1.Role_ROLE_APPLICANT},
			{Email: "b@cst.edu.bt", Role: authv1.Role_ROLE_ADMIN},
		},
		Pagination: &commonv1.PaginationResponse{NextPageToken: "100", TotalCount: 150},
	}}
	emails, next, err := clients.Auth{API: api}.ListAdmins(context.Background(), "0")
	require.NoError(t, err)
	assert.Equal(t, []string{"a@cst.edu.bt", "b@cst.edu.bt"}, emails)
	assert.Equal(t, "100", next)
	assert.Equal(t, authv1.Role_ROLE_ADMIN, api.got.GetRole())
	assert.Equal(t, int32(clients.AdminPageSize), api.got.GetPagination().PageSize)
	assert.Equal(t, "0", api.got.GetPagination().PageToken)
}

func TestAuth_ListAdmins_LastPageAndErrors(t *testing.T) {
	emails, next, err := clients.Auth{API: &fakeAuth{resp: &authv1.ListUsersByRoleResponse{}}}.ListAdmins(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, emails)
	assert.Empty(t, next)

	rpcErr := errors.New("permission denied")
	_, _, err = clients.Auth{API: &fakeAuth{err: rpcErr}}.ListAdmins(context.Background(), "")
	assert.ErrorIs(t, err, rpcErr)
}
