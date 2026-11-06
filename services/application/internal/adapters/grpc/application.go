package grpc

import (
	"context"
	applicationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ApplicationHandler struct {
	applicationv1.UnimplementedApplicationServiceServer
	service *app.Service
}

func NewApplicationHandler(service *app.Service) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}
func message(a domain.Application) *applicationv1.Application {
	return &applicationv1.Application{ApplicationId: a.ID, ApplicantId: a.ApplicantID, JobId: a.JobID, ResumeId: a.ResumeID, EmployerId: a.EmployerID, Withdrawn: a.Withdrawn, Timestamps: &commonv1.ResourceTimestamps{CreatedAt: timestamppb.New(a.CreatedAt), UpdatedAt: timestamppb.New(a.UpdatedAt)}}
}
func (h *ApplicationHandler) SubmitApplication(ctx context.Context, r *applicationv1.SubmitApplicationRequest) (*applicationv1.SubmitApplicationResponse, error) {
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	a, err := h.service.Submit(ctx, r.JobId, r.ResumeId)
	if err != nil {
		return nil, err
	}
	return &applicationv1.SubmitApplicationResponse{Application: message(a)}, nil
}
func (h *ApplicationHandler) WithdrawApplication(ctx context.Context, r *applicationv1.WithdrawApplicationRequest) (*applicationv1.WithdrawApplicationResponse, error) {
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	a, err := h.service.Withdraw(ctx, r.ApplicationId)
	if err != nil {
		return nil, err
	}
	return &applicationv1.WithdrawApplicationResponse{Application: message(a)}, nil
}
func (h *ApplicationHandler) GetApplication(ctx context.Context, r *applicationv1.GetApplicationRequest) (*applicationv1.GetApplicationResponse, error) {
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	a, err := h.service.Get(ctx, r.ApplicationId)
	if err != nil {
		return nil, err
	}
	return &applicationv1.GetApplicationResponse{Application: message(a)}, nil
}
func (h *ApplicationHandler) ListApplications(ctx context.Context, r *applicationv1.ListApplicationsRequest) (*applicationv1.ListApplicationsResponse, error) {
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	rows, total, next, err := h.service.List(ctx, r.JobId, r.GetPagination().GetPageSize(), r.GetPagination().GetPageToken())
	if err != nil {
		return nil, err
	}
	result := &applicationv1.ListApplicationsResponse{Applications: make([]*applicationv1.Application, 0, len(rows)), Pagination: &commonv1.PaginationResponse{TotalCount: total, NextPageToken: next}}
	for _, a := range rows {
		result.Applications = append(result.Applications, message(a))
	}
	return result, nil
}
