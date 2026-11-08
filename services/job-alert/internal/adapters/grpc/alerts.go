package grpc

import (
	"context"
	"errors"
	"strconv"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobalertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AlertHandler struct {
	jobalertv1.UnimplementedJobAlertServiceServer
	alerts *app.Alerts
}

func NewAlertHandler(alerts *app.Alerts) *AlertHandler { return &AlertHandler{alerts: alerts} }
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}
func (h *AlertHandler) CreateJobAlert(ctx context.Context, req *jobalertv1.CreateJobAlertRequest) (*jobalertv1.CreateJobAlertResponse, error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	a, err := h.alerts.Create(ctx, c, req.GetCategoryIds(), req.GetTagIds(), req.GetLocation())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobalertv1.CreateJobAlertResponse{JobAlert: alertToProto(a)}, nil
}
func (h *AlertHandler) ListMyJobAlerts(ctx context.Context, req *jobalertv1.ListMyJobAlertsRequest) (*jobalertv1.ListMyJobAlertsResponse, error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	alerts, total, page, err := h.alerts.List(ctx, c, req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	out := &jobalertv1.ListMyJobAlertsResponse{JobAlerts: make([]*jobalertv1.JobAlert, 0, len(alerts)), Pagination: &commonv1.PaginationResponse{TotalCount: total}}
	for _, a := range alerts {
		out.JobAlerts = append(out.JobAlerts, alertToProto(a))
	}
	// Subtract before adding to avoid overflow for a large valid offset.
	if page.Offset < total && int64(len(alerts)) < total-page.Offset {
		out.Pagination.NextPageToken = strconv.FormatInt(page.Offset+int64(len(alerts)), 10)
	}
	return out, nil
}
func (h *AlertHandler) DeleteJobAlert(ctx context.Context, req *jobalertv1.DeleteJobAlertRequest) (*jobalertv1.DeleteJobAlertResponse, error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	if err = h.alerts.Delete(ctx, c, req.GetAlertId()); err != nil {
		return nil, toStatus(err)
	}
	return &jobalertv1.DeleteJobAlertResponse{}, nil
}
func alertToProto(a domain.Alert) *jobalertv1.JobAlert {
	return &jobalertv1.JobAlert{AlertId: a.ID, ApplicantId: a.ApplicantID, CategoryIds: a.CategoryIDs, TagIds: a.TagIDs, Location: a.Location, Timestamps: &commonv1.ResourceTimestamps{CreatedAt: timestamppb.New(a.CreatedAt), UpdatedAt: timestamppb.New(a.UpdatedAt)}}
}
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrPagination):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrLimit):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrResolverUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "subscription operation failed")
	}
}
