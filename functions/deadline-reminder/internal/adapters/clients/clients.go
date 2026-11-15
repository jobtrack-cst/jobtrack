// Package clients adapts the job-posting and saved-jobs internal gRPC clients
// to the deadline-reminder ports. Both connections carry the JT-44 internal
// credentials through grpcclient.InternalUnaryClientInterceptor (set in main).
package clients

import (
	"context"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/ports"
)

// PageSize is the largest page the services return (AGENTS.md Section 9).
const PageSize = 100

// JobPostingAPI is the part of jobpostingv1.JobPostingInternalServiceClient we use.
type JobPostingAPI interface {
	ListJobsClosingBetween(ctx context.Context, in *jobpostingv1.ListJobsClosingBetweenRequest, opts ...grpc.CallOption) (*jobpostingv1.ListJobsClosingBetweenResponse, error)
}

// JobPosting implements ports.Jobs.
type JobPosting struct{ API JobPostingAPI }

var _ ports.Jobs = JobPosting{}

func (j JobPosting) ListClosing(ctx context.Context, start, end time.Time, pageToken string) ([]domain.ClosingJob, string, error) {
	resp, err := j.API.ListJobsClosingBetween(ctx, &jobpostingv1.ListJobsClosingBetweenRequest{
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(end),
		Pagination: &commonv1.PaginationRequest{PageSize: PageSize, PageToken: pageToken},
	})
	if err != nil {
		return nil, "", err
	}
	jobs := make([]domain.ClosingJob, 0, len(resp.GetJobs()))
	for _, job := range resp.GetJobs() {
		var deadline time.Time
		if job.GetDeadline() != nil && job.GetDeadline().CheckValid() == nil {
			deadline = job.GetDeadline().AsTime()
		}
		jobs = append(jobs, domain.ClosingJob{ID: job.GetJobId(), Deadline: deadline})
	}
	return jobs, resp.GetPagination().GetNextPageToken(), nil
}

// SavedJobsAPI is the part of savedjobsv1.SavedJobsInternalServiceClient we use.
type SavedJobsAPI interface {
	ListJobSavers(ctx context.Context, in *savedjobsv1.ListJobSaversRequest, opts ...grpc.CallOption) (*savedjobsv1.ListJobSaversResponse, error)
}

// SavedJobs implements ports.Savers.
type SavedJobs struct{ API SavedJobsAPI }

var _ ports.Savers = SavedJobs{}

func (s SavedJobs) ListSavers(ctx context.Context, jobID, pageToken string) ([]string, string, error) {
	resp, err := s.API.ListJobSavers(ctx, &savedjobsv1.ListJobSaversRequest{
		JobId:      jobID,
		Pagination: &commonv1.PaginationRequest{PageSize: PageSize, PageToken: pageToken},
	})
	if err != nil {
		return nil, "", err
	}
	return resp.GetApplicantIds(), resp.GetPagination().GetNextPageToken(), nil
}
