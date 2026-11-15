// Package clients adapts the analytics and auth internal gRPC clients to the
// weekly summary ports.
package clients

import (
	"context"
	"errors"
	"time"

	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
)

// AdminPageSize is the largest page auth returns (AGENTS.md Section 9).
const AdminPageSize = 100

var errNoSummary = errors.New("analytics returned no summary")

// AnalyticsAPI is the part of analyticsv1.AnalyticsInternalServiceClient we use.
type AnalyticsAPI interface {
	GetInternalSummary(ctx context.Context, in *analyticsv1.GetInternalSummaryRequest, opts ...grpc.CallOption) (*analyticsv1.GetInternalSummaryResponse, error)
}

// Analytics implements ports.Stats.
type Analytics struct{ API AnalyticsAPI }

func (a Analytics) Summary(ctx context.Context, start, end time.Time) (domain.Summary, error) {
	resp, err := a.API.GetInternalSummary(ctx, &analyticsv1.GetInternalSummaryRequest{
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(end),
	})
	if err != nil {
		return domain.Summary{}, err
	}
	s := resp.GetSummary()
	if s == nil {
		return domain.Summary{}, errNoSummary
	}
	out := domain.Summary{
		Start: s.GetStartTime().AsTime(), End: s.GetEndTime().AsTime(),
		JobsPublished: s.GetJobsPublished(), JobsClosed: s.GetJobsClosed(),
		ApplicationsSubmitted: s.GetApplicationsSubmitted(), ApplicationsWithdrawn: s.GetApplicationsWithdrawn(),
		OffersMade: s.GetOffersMade(), OffersAccepted: s.GetOffersAccepted(), OffersDeclined: s.GetOffersDeclined(),
	}
	for _, c := range s.GetStatusChanges() {
		out.StatusChanges = append(out.StatusChanges, domain.StatusCount{Status: c.GetStatus(), Count: c.GetCount()})
	}
	return out, nil
}

// AuthAPI is the part of authv1.AuthInternalServiceClient we use.
type AuthAPI interface {
	ListUsersByRole(ctx context.Context, in *authv1.ListUsersByRoleRequest, opts ...grpc.CallOption) (*authv1.ListUsersByRoleResponse, error)
}

// Auth implements ports.Admins.
type Auth struct{ API AuthAPI }

// ListAdmins returns one page of admin emails. Users whose role is not admin
// are skipped, so a wrong response can never email an applicant or employer.
func (a Auth) ListAdmins(ctx context.Context, pageToken string) ([]string, string, error) {
	resp, err := a.API.ListUsersByRole(ctx, &authv1.ListUsersByRoleRequest{
		Role:       authv1.Role_ROLE_ADMIN,
		Pagination: &commonv1.PaginationRequest{PageSize: AdminPageSize, PageToken: pageToken},
	})
	if err != nil {
		return nil, "", err
	}
	var emails []string
	for _, u := range resp.GetUsers() {
		if u.GetRole() == authv1.Role_ROLE_ADMIN {
			emails = append(emails, u.GetEmail())
		}
	}
	return emails, resp.GetPagination().GetNextPageToken(), nil
}
