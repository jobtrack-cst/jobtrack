package grpc

import (
	"context"
	"errors"
	"time"

	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

// AnalyticsHandler serves the public AnalyticsService through the gateway.
type AnalyticsHandler struct {
	analyticsv1.UnimplementedAnalyticsServiceServer
	analytics *app.Analytics
}

var _ analyticsv1.AnalyticsServiceServer = (*AnalyticsHandler)(nil)

func NewAnalyticsHandler(analytics *app.Analytics) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

// GetSummary is admin only. The generated getters are nil-safe, so a nil
// request has no times and is INVALID_ARGUMENT for an admin.
func (h *AnalyticsHandler) GetSummary(ctx context.Context, req *analyticsv1.GetSummaryRequest) (*analyticsv1.GetSummaryResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	// Timestamps are converted here, before the app, so the role is checked
	// first: a non-admin must not learn which timestamps are valid.
	if err := caller.RequireAdmin(); err != nil {
		return nil, toStatus(err)
	}
	in, err := summaryInput(req.GetStartTime(), req.GetEndTime())
	if err != nil {
		return nil, toStatus(err)
	}
	summary, err := h.analytics.GetSummary(ctx, caller, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &analyticsv1.GetSummaryResponse{Summary: summaryToProto(summary, in)}, nil
}

// AnalyticsInternalHandler serves AnalyticsInternalService. The gateway never
// registers it, and the internal server interceptor (pkg/authctx) has
// already checked INTERNAL_TOKEN and that the caller is weekly-summary, so
// there is no user role to check here.
type AnalyticsInternalHandler struct {
	analyticsv1.UnimplementedAnalyticsInternalServiceServer
	analytics *app.Analytics
}

var _ analyticsv1.AnalyticsInternalServiceServer = (*AnalyticsInternalHandler)(nil)

func NewAnalyticsInternalHandler(analytics *app.Analytics) *AnalyticsInternalHandler {
	return &AnalyticsInternalHandler{analytics: analytics}
}

func (h *AnalyticsInternalHandler) GetInternalSummary(ctx context.Context, req *analyticsv1.GetInternalSummaryRequest) (*analyticsv1.GetInternalSummaryResponse, error) {
	in, err := summaryInput(req.GetStartTime(), req.GetEndTime())
	if err != nil {
		return nil, toStatus(err)
	}
	summary, err := h.analytics.GetInternalSummary(ctx, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &analyticsv1.GetInternalSummaryResponse{Summary: summaryToProto(summary, in)}, nil
}

// callerFrom trusts x-user-id and x-user-role only because the gateway sets
// them and strips any client-sent values (JT-31).
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

// summaryInput converts both request times. A missing time stays nil, and
// the domain rejects it, so both RPCs share one rule.
func summaryInput(start, end *timestamppb.Timestamp) (app.SummaryInput, error) {
	s, err := optionalTime(start)
	if err != nil {
		return app.SummaryInput{}, err
	}
	e, err := optionalTime(end)
	if err != nil {
		return app.SummaryInput{}, err
	}
	return app.SummaryInput{Start: s, End: e}, nil
}

// optionalTime returns nil for an unset field. CheckValid rejects seconds
// outside years 1 to 9999 and nanos outside 0 to 999999999.
func optionalTime(ts *timestamppb.Timestamp) (*time.Time, error) {
	if ts == nil {
		return nil, nil
	}
	if err := ts.CheckValid(); err != nil {
		return nil, domain.ErrInvalidTimestamp
	}
	t := ts.AsTime()
	return &t, nil
}

// summaryToProto echoes the request times exactly as sent. in has both times
// set, because the app accepted the range.
func summaryToProto(s domain.Summary, in app.SummaryInput) *analyticsv1.Summary {
	changes := make([]*analyticsv1.StatusCount, 0, len(s.StatusChanges))
	for _, c := range s.StatusChanges {
		changes = append(changes, &analyticsv1.StatusCount{Status: c.Status, Count: c.Count})
	}
	return &analyticsv1.Summary{
		StartTime:             timestamppb.New(*in.Start),
		EndTime:               timestamppb.New(*in.End),
		JobsPublished:         s.JobsPublished,
		JobsClosed:            s.JobsClosed,
		ApplicationsSubmitted: s.ApplicationsSubmitted,
		ApplicationsWithdrawn: s.ApplicationsWithdrawn,
		OffersMade:            s.OffersMade,
		OffersAccepted:        s.OffersAccepted,
		OffersDeclined:        s.OffersDeclined,
		StatusChanges:         changes,
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidTimestamp), errors.Is(err, domain.ErrInvalidTimeRange):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrAdminRoleRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
