package clients

import (
	"context"

	jobv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Connections must install grpcclient.UnaryClientInterceptor to forward identity.
type JobPosting struct{ client jobv1.JobPostingServiceClient }
type Resume struct{ client resumev1.ResumeServiceClient }

var _ ports.JobPosting = (*JobPosting)(nil)
var _ ports.Resume = (*Resume)(nil)

func NewJobPosting(client jobv1.JobPostingServiceClient) *JobPosting {
	return &JobPosting{client: client}
}
func NewResume(client resumev1.ResumeServiceClient) *Resume { return &Resume{client: client} }

func (c *JobPosting) GetJob(ctx context.Context, id string) (domain.Job, error) {
	r, err := c.client.GetJob(ctx, &jobv1.GetJobRequest{JobId: id})
	if err != nil {
		return domain.Job{}, err
	}
	j := r.GetJob()
	if j == nil || j.GetJobId() != id || !domain.ValidID(j.GetEmployerId()) || j.GetDeadline() == nil || j.GetDeadline().CheckValid() != nil {
		return domain.Job{}, status.Error(codes.Internal, "invalid job dependency response")
	}
	switch j.GetStatus() {
	case jobv1.JobStatus_JOB_STATUS_DRAFT, jobv1.JobStatus_JOB_STATUS_PUBLISHED, jobv1.JobStatus_JOB_STATUS_CLOSED, jobv1.JobStatus_JOB_STATUS_REMOVED:
	default:
		return domain.Job{}, status.Error(codes.Internal, "invalid job dependency response")
	}
	return domain.Job{ID: j.JobId, EmployerID: j.EmployerId, Published: j.Status == jobv1.JobStatus_JOB_STATUS_PUBLISHED, Deadline: j.Deadline.AsTime()}, nil
}
func (c *Resume) GetResume(ctx context.Context, id string) (domain.Resume, error) {
	r, err := c.client.GetResume(ctx, &resumev1.GetResumeRequest{ResumeId: id})
	if err != nil {
		return domain.Resume{}, err
	}
	v := r.GetResume()
	if v == nil || v.GetResumeId() != id || !domain.ValidID(v.GetApplicantId()) {
		return domain.Resume{}, status.Error(codes.Internal, "invalid resume dependency response")
	}
	switch v.GetValidationStatus() {
	case resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_PENDING, resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED, resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED:
	default:
		return domain.Resume{}, status.Error(codes.Internal, "invalid resume dependency response")
	}
	return domain.Resume{ID: v.ResumeId, ApplicantID: v.ApplicantId, Validated: v.ValidationStatus == resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED}, nil
}
