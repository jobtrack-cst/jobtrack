package grpc

import (
	"context"
	"errors"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
)

// JobPostingHandler serves the public JobPostingService through the gateway,
// and GetJob for application with the user's identity forwarded.
type JobPostingHandler struct {
	jobpostingv1.UnimplementedJobPostingServiceServer
	jobs *app.Jobs
}

var _ jobpostingv1.JobPostingServiceServer = (*JobPostingHandler)(nil)

func NewJobPostingHandler(jobs *app.Jobs) *JobPostingHandler {
	return &JobPostingHandler{jobs: jobs}
}

// CreateJob is employer only. The generated getters are nil-safe, so a nil
// request is an empty one and fails validation.
func (h *JobPostingHandler) CreateJob(ctx context.Context, req *jobpostingv1.CreateJobRequest) (*jobpostingv1.CreateJobResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	in, err := jobInput(req.GetTitle(), req.GetDescription(), req.GetCategoryId(), req.GetTagIds(), req.GetLocation(), req.GetDeadline())
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.jobs.Create(ctx, caller, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.CreateJobResponse{Job: jobToProto(job)}, nil
}

func (h *JobPostingHandler) UpdateJob(ctx context.Context, req *jobpostingv1.UpdateJobRequest) (*jobpostingv1.UpdateJobResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	in, err := jobInput(req.GetTitle(), req.GetDescription(), req.GetCategoryId(), req.GetTagIds(), req.GetLocation(), req.GetDeadline())
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.jobs.Update(ctx, caller, req.GetJobId(), in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.UpdateJobResponse{Job: jobToProto(job)}, nil
}

func (h *JobPostingHandler) PublishJob(ctx context.Context, req *jobpostingv1.PublishJobRequest) (*jobpostingv1.PublishJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.jobs.Publish(ctx, caller, req.GetJobId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.PublishJobResponse{Job: jobToProto(job)}, nil
}

func (h *JobPostingHandler) CloseJob(ctx context.Context, req *jobpostingv1.CloseJobRequest) (*jobpostingv1.CloseJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.jobs.Close(ctx, caller, req.GetJobId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.CloseJobResponse{Job: jobToProto(job)}, nil
}

func (h *JobPostingHandler) GetJob(ctx context.Context, req *jobpostingv1.GetJobRequest) (*jobpostingv1.GetJobResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	job, err := h.jobs.Get(ctx, caller, req.GetJobId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.GetJobResponse{Job: jobToProto(job)}, nil
}

func (h *JobPostingHandler) ListMyJobs(ctx context.Context, req *jobpostingv1.ListMyJobsRequest) (*jobpostingv1.ListMyJobsResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	filter, err := statusFilter(req.GetStatus())
	if err != nil {
		return nil, toStatus(err)
	}
	page, err := h.jobs.ListMine(ctx, caller, filter, req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.ListMyJobsResponse{Jobs: jobsToProto(page.Jobs), Pagination: pageToProto(page)}, nil
}

// JobPostingInternalHandler serves JobPostingInternalService. The gateway
// never registers it, and the internal server interceptor (pkg/authctx) has
// already checked INTERNAL_TOKEN and that the caller is deadline-reminder, so
// there is no user role to check here.
type JobPostingInternalHandler struct {
	jobpostingv1.UnimplementedJobPostingInternalServiceServer
	jobs *app.Jobs
}

var _ jobpostingv1.JobPostingInternalServiceServer = (*JobPostingInternalHandler)(nil)

func NewJobPostingInternalHandler(jobs *app.Jobs) *JobPostingInternalHandler {
	return &JobPostingInternalHandler{jobs: jobs}
}

func (h *JobPostingInternalHandler) ListJobsClosingBetween(ctx context.Context, req *jobpostingv1.ListJobsClosingBetweenRequest) (*jobpostingv1.ListJobsClosingBetweenResponse, error) {
	start, err := optionalTime(req.GetStartTime(), domain.ErrInvalidTimeRange)
	if err != nil {
		return nil, toStatus(err)
	}
	end, err := optionalTime(req.GetEndTime(), domain.ErrInvalidTimeRange)
	if err != nil {
		return nil, toStatus(err)
	}
	page, err := h.jobs.ListClosingBetween(ctx, start, end, req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobpostingv1.ListJobsClosingBetweenResponse{Jobs: jobsToProto(page.Jobs), Pagination: pageToProto(page)}, nil
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

// employerFrom checks the role before the handler converts any input, so a
// non-employer learns nothing about which inputs are valid.
func employerFrom(ctx context.Context) (domain.Caller, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireEmployer(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

func jobInput(title, description, categoryID string, tagIDs []string, location string, deadline *timestamppb.Timestamp) (domain.JobInput, error) {
	d, err := optionalTime(deadline, domain.ErrInvalidDeadline)
	if err != nil {
		return domain.JobInput{}, err
	}
	return domain.JobInput{
		Title: title, Description: description, CategoryID: categoryID,
		TagIDs: tagIDs, Location: location, Deadline: d,
	}, nil
}

// optionalTime returns the zero time for an unset field, which the domain
// rejects. CheckValid rejects seconds outside years 1 to 9999 and nanos
// outside 0 to 999999999.
func optionalTime(ts *timestamppb.Timestamp, invalid error) (time.Time, error) {
	if ts == nil {
		return time.Time{}, nil
	}
	if err := ts.CheckValid(); err != nil {
		return time.Time{}, invalid
	}
	return ts.AsTime(), nil
}

// statusFilter maps the ListMyJobs filter. UNSPECIFIED means all statuses;
// a number outside the enum is INVALID_ARGUMENT.
func statusFilter(s jobpostingv1.JobStatus) (domain.Status, error) {
	switch s {
	case jobpostingv1.JobStatus_JOB_STATUS_UNSPECIFIED:
		return domain.StatusUnspecified, nil
	case jobpostingv1.JobStatus_JOB_STATUS_DRAFT:
		return domain.StatusDraft, nil
	case jobpostingv1.JobStatus_JOB_STATUS_PUBLISHED:
		return domain.StatusPublished, nil
	case jobpostingv1.JobStatus_JOB_STATUS_CLOSED:
		return domain.StatusClosed, nil
	case jobpostingv1.JobStatus_JOB_STATUS_REMOVED:
		return domain.StatusRemoved, nil
	default:
		return domain.StatusUnspecified, domain.ErrInvalidStatusFilter
	}
}

func statusToProto(s domain.Status) jobpostingv1.JobStatus {
	switch s {
	case domain.StatusDraft:
		return jobpostingv1.JobStatus_JOB_STATUS_DRAFT
	case domain.StatusPublished:
		return jobpostingv1.JobStatus_JOB_STATUS_PUBLISHED
	case domain.StatusClosed:
		return jobpostingv1.JobStatus_JOB_STATUS_CLOSED
	case domain.StatusRemoved:
		return jobpostingv1.JobStatus_JOB_STATUS_REMOVED
	default:
		return jobpostingv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func jobToProto(j domain.Job) *jobpostingv1.Job {
	return &jobpostingv1.Job{
		JobId:       j.ID,
		EmployerId:  j.EmployerID,
		Title:       j.Title,
		Description: j.Description,
		CategoryId:  j.CategoryID,
		TagIds:      j.TagIDs,
		Location:    j.Location,
		Deadline:    timestamppb.New(j.Deadline),
		Status:      statusToProto(j.Status),
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(j.CreatedAt),
			UpdatedAt: timestamppb.New(j.UpdatedAt),
		},
	}
}

func jobsToProto(jobs []domain.Job) []*jobpostingv1.Job {
	out := make([]*jobpostingv1.Job, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToProto(j))
	}
	return out
}

func pageToProto(p app.JobPage) *commonv1.PaginationResponse {
	return &commonv1.PaginationResponse{NextPageToken: p.NextPageToken, TotalCount: int64(p.TotalCount)}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidJobID), errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrInvalidDescription), errors.Is(err, domain.ErrInvalidCategoryID),
		errors.Is(err, domain.ErrInvalidTagIDs), errors.Is(err, domain.ErrInvalidLocation),
		errors.Is(err, domain.ErrInvalidDeadline), errors.Is(err, domain.ErrInvalidStatusFilter),
		errors.Is(err, domain.ErrInvalidTimeRange), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrEmployerRoleRequired), errors.Is(err, domain.ErrNotJobOwner):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrJobNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrJobNotEditable), errors.Is(err, domain.ErrJobNotDraft),
		errors.Is(err, domain.ErrJobNotPublished), errors.Is(err, domain.ErrDeadlinePassed),
		errors.Is(err, domain.ErrEmployerNotVerified), errors.Is(err, domain.ErrJobStatusChanged):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrEmployerCheckUnavailable):
		return status.Error(codes.Internal, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
