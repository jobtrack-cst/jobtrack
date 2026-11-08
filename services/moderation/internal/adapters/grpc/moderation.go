package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	moderationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/moderation/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

// ModerationHandler serves the public ModerationService through the gateway.
// There is no internal service in the contract.
type ModerationHandler struct {
	moderationv1.UnimplementedModerationServiceServer
	reports *app.Reports
}

var _ moderationv1.ModerationServiceServer = (*ModerationHandler)(nil)

func NewModerationHandler(reports *app.Reports) *ModerationHandler {
	return &ModerationHandler{reports: reports}
}

// The generated getters are nil-safe, so a nil request reaches the app as
// empty fields. That keeps the role check first on the admin RPCs, and the
// empty fields are then INVALID_ARGUMENT.

func (h *ModerationHandler) ReportJob(ctx context.Context, req *moderationv1.ReportJobRequest) (*moderationv1.ReportJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	r, err := h.reports.Report(ctx, caller, req.GetJobId(), req.GetReason())
	if err != nil {
		return nil, toStatus(err)
	}
	return &moderationv1.ReportJobResponse{Report: reportToProto(r)}, nil
}

func (h *ModerationHandler) ListReports(ctx context.Context, req *moderationv1.ListReportsRequest) (*moderationv1.ListReportsResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	// A missing pagination message means the defaults, so nil is fine here.
	// The enum number goes to the app unchanged; domain.ParseStatusFilter
	// rejects unknown numbers after the role check.
	p := req.GetPagination()
	result, err := h.reports.List(ctx, caller, int32(req.GetStatus()), p.GetPageSize(), p.GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &moderationv1.ListReportsResponse{
		Reports:    reportsToProto(result.Reports),
		Pagination: &commonv1.PaginationResponse{NextPageToken: result.NextPageToken, TotalCount: result.TotalCount},
	}, nil
}

func (h *ModerationHandler) DismissReport(ctx context.Context, req *moderationv1.DismissReportRequest) (*moderationv1.DismissReportResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	r, err := h.reports.Dismiss(ctx, caller, req.GetReportId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &moderationv1.DismissReportResponse{Report: reportToProto(r)}, nil
}

// RemoveJob publishes moderation.job_removed in Sprint 2.
func (h *ModerationHandler) RemoveJob(ctx context.Context, req *moderationv1.RemoveJobRequest) (*moderationv1.RemoveJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	jobID, err := h.reports.RemoveJob(ctx, caller, req.GetJobId(), req.GetReason())
	if err != nil {
		return nil, toStatus(err)
	}
	return &moderationv1.RemoveJobResponse{JobId: jobID}, nil
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

func statusToProto(s domain.ReportStatus) moderationv1.ReportStatus {
	switch s {
	case domain.StatusOpen:
		return moderationv1.ReportStatus_REPORT_STATUS_OPEN
	case domain.StatusDismissed:
		return moderationv1.ReportStatus_REPORT_STATUS_DISMISSED
	case domain.StatusActioned:
		return moderationv1.ReportStatus_REPORT_STATUS_ACTIONED
	default:
		return moderationv1.ReportStatus_REPORT_STATUS_UNSPECIFIED
	}
}

func reportToProto(r domain.Report) *moderationv1.Report {
	return &moderationv1.Report{
		ReportId:   r.ID,
		JobId:      r.JobID,
		ReporterId: r.ReporterID,
		Reason:     r.Reason,
		Status:     statusToProto(r.Status),
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(r.CreatedAt),
			UpdatedAt: timestamppb.New(r.UpdatedAt),
		},
	}
}

func reportsToProto(items []domain.Report) []*moderationv1.Report {
	out := make([]*moderationv1.Report, 0, len(items))
	for _, r := range items {
		out = append(out, reportToProto(r))
	}
	return out
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidJobID), errors.Is(err, domain.ErrInvalidReportID),
		errors.Is(err, domain.ErrInvalidReportReason), errors.Is(err, domain.ErrInvalidRemovalReason),
		errors.Is(err, domain.ErrInvalidStatusFilter), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrAdminRoleRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrReportNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrOpenReportExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrJobAlreadyRemoved), errors.Is(err, domain.ErrReportNotOpen):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
