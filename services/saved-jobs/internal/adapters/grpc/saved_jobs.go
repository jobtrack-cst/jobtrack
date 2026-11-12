package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

type SavedJobsHandler struct {
	savedjobsv1.UnimplementedSavedJobsServiceServer
	svc *app.SavedJobs
}

var _ savedjobsv1.SavedJobsServiceServer = (*SavedJobsHandler)(nil)

func NewSavedJobsHandler(svc *app.SavedJobs) *SavedJobsHandler {
	return &SavedJobsHandler{svc: svc}
}

func (h *SavedJobsHandler) SaveJob(ctx context.Context, req *savedjobsv1.SaveJobRequest) (*savedjobsv1.SaveJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.svc.SaveJob(ctx, caller, req.GetJobId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &savedjobsv1.SaveJobResponse{SavedJob: toProto(job)}, nil
}

func (h *SavedJobsHandler) UnsaveJob(ctx context.Context, req *savedjobsv1.UnsaveJobRequest) (*savedjobsv1.UnsaveJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if err := h.svc.UnsaveJob(ctx, caller, req.GetJobId()); err != nil {
		return nil, toStatus(err)
	}
	return &savedjobsv1.UnsaveJobResponse{}, nil
}

func (h *SavedJobsHandler) ListMySavedJobs(ctx context.Context, req *savedjobsv1.ListMySavedJobsRequest) (*savedjobsv1.ListMySavedJobsResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	pageSize := int(req.GetPagination().GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}
	// Simple offset for Sprint 1
	offset := 0

	page, err := h.svc.ListMySavedJobs(ctx, caller, pageSize, offset)
	if err != nil {
		return nil, toStatus(err)
	}

	var jobs []*savedjobsv1.SavedJob
	for _, j := range page.Jobs {
		jobs = append(jobs, toProto(j))
	}

	return &savedjobsv1.ListMySavedJobsResponse{
		SavedJobs:  jobs,
		Pagination: &commonv1.PaginationResponse{TotalCount: int64(page.Total)},
	}, nil
}

// Internal Handler
type SavedJobsInternalHandler struct {
	savedjobsv1.UnimplementedSavedJobsInternalServiceServer
	svc *app.SavedJobs
}

var _ savedjobsv1.SavedJobsInternalServiceServer = (*SavedJobsInternalHandler)(nil)

func NewSavedJobsInternalHandler(svc *app.SavedJobs) *SavedJobsInternalHandler {
	return &SavedJobsInternalHandler{svc: svc}
}

func (h *SavedJobsInternalHandler) ListJobSavers(ctx context.Context, req *savedjobsv1.ListJobSaversRequest) (*savedjobsv1.ListJobSaversResponse, error) {
	// No role check here; the gRPC interceptor handles the INTERNAL_TOKEN check.
	pageSize := int(req.GetPagination().GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}
	offset := 0

	savers, total, err := h.svc.ListJobSavers(ctx, req.GetJobId(), pageSize, offset)
	if err != nil {
		return nil, toStatus(err)
	}

	return &savedjobsv1.ListJobSaversResponse{
		ApplicantIds: savers,
		Pagination:   &commonv1.PaginationResponse{TotalCount: int64(total)},
	}, nil
}

// Helpers
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

func toProto(j domain.SavedJob) *savedjobsv1.SavedJob {
	return &savedjobsv1.SavedJob{
		JobId:       j.JobID,
		ApplicantId: j.ApplicantID,
		SavedAt:     timestamppb.New(j.SavedAt),
	}
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidJobID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrApplicantOnly):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAlreadySaved):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrNotSaved):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrJobClosed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
