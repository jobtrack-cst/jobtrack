package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	repo    ports.Repository
	jobs    ports.JobPosting
	resumes ports.Resume
	now     func() time.Time
}

func NewService(repo ports.Repository, jobs ports.JobPosting, resumes ports.Resume) *Service {
	return &Service{repo: repo, jobs: jobs, resumes: resumes, now: time.Now}
}
func identity(ctx context.Context, write bool) (authctx.Identity, error) {
	i, ok := authctx.FromContext(ctx)
	if !ok || !domain.ValidID(i.UserID) {
		return i, status.Error(codes.Unauthenticated, "valid caller identity required")
	}
	if i.Role != "applicant" && (write || (i.Role != "employer" && i.Role != "admin")) {
		return i, status.Error(codes.PermissionDenied, "role is not allowed")
	}
	return i, nil
}
func validID(id string) error {
	if !domain.ValidID(id) {
		return status.Error(codes.InvalidArgument, "canonical nonzero UUID required")
	}
	return nil
}
func mapped(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "application not found")
	case errors.Is(err, domain.ErrDuplicate):
		return status.Error(codes.AlreadyExists, "application already exists for this job")
	case errors.Is(err, domain.ErrWithdrawn):
		return status.Error(codes.FailedPrecondition, "application already withdrawn")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "application storage failed")
	}
}
func dependencyError(err error) error {
	switch status.Code(err) {
	case codes.NotFound, codes.PermissionDenied, codes.FailedPrecondition, codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return status.Error(status.Code(err), "eligibility dependency rejected or could not complete the request")
	default:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return mapped(err)
		}
		return status.Error(codes.Internal, "eligibility dependency failed")
	}
}
func (s *Service) Submit(ctx context.Context, jobID, resumeID string) (domain.Application, error) {
	var zero domain.Application
	i, err := identity(ctx, true)
	if err != nil {
		return zero, err
	}
	for _, id := range []string{jobID, resumeID} {
		if err = validID(id); err != nil {
			return zero, err
		}
	}
	// Existing applications take precedence over eligibility that may have changed.
	existing, _, err := s.repo.List(ctx, ports.Filter{ApplicantID: i.UserID, JobID: jobID, Limit: 1})
	if err != nil {
		return zero, mapped(err)
	}
	if len(existing) != 0 {
		return zero, mapped(domain.ErrDuplicate)
	}
	if s.jobs == nil || s.resumes == nil {
		return zero, status.Error(codes.Unavailable, "eligibility clients are deferred to Sprint 2")
	}
	job, err := s.jobs.GetJob(ctx, jobID)
	if err != nil {
		return zero, dependencyError(err)
	}
	if job.ID != jobID || !domain.ValidID(job.EmployerID) || job.Deadline.IsZero() {
		return zero, status.Error(codes.Internal, "invalid job dependency response")
	}
	if !job.Eligible(s.now()) {
		return zero, status.Error(codes.FailedPrecondition, "job must be published and before deadline")
	}
	resume, err := s.resumes.GetResume(ctx, resumeID)
	if err != nil {
		return zero, dependencyError(err)
	}
	if resume.ID != resumeID {
		return zero, status.Error(codes.Internal, "invalid resume dependency response")
	}
	if resume.ApplicantID != i.UserID {
		return zero, status.Error(codes.PermissionDenied, "resume belongs to another applicant")
	}
	if !resume.Validated {
		return zero, status.Error(codes.FailedPrecondition, "resume must be validated")
	}
	now := s.now().UTC()
	a, err := s.repo.Create(ctx, domain.Application{ID: uuid.NewString(), ApplicantID: i.UserID, JobID: jobID, ResumeID: resumeID, EmployerID: job.EmployerID, CreatedAt: now, UpdatedAt: now})
	return a, mapped(err)
}
func (s *Service) Get(ctx context.Context, id string) (domain.Application, error) {
	var zero domain.Application
	i, err := identity(ctx, false)
	if err != nil {
		return zero, err
	}
	if err = validID(id); err != nil {
		return zero, err
	}
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return zero, mapped(err)
	}
	if (i.Role == "applicant" && a.ApplicantID != i.UserID) || (i.Role == "employer" && a.EmployerID != i.UserID) {
		return zero, status.Error(codes.PermissionDenied, "application belongs to another user")
	}
	return a, nil
}
func (s *Service) Withdraw(ctx context.Context, id string) (domain.Application, error) {
	var zero domain.Application
	i, err := identity(ctx, true)
	if err != nil {
		return zero, err
	}
	a, err := s.Get(ctx, id)
	if err != nil {
		return zero, err
	}
	if a.Withdrawn {
		return zero, status.Error(codes.FailedPrecondition, "application already withdrawn")
	}
	a, err = s.repo.Withdraw(ctx, id, i.UserID)
	return a, mapped(err)
}
func (s *Service) List(ctx context.Context, jobID string, size int32, token string) ([]domain.Application, int64, string, error) {
	i, err := identity(ctx, false)
	if err != nil {
		return nil, 0, "", err
	}
	if jobID != "" {
		if err = validID(jobID); err != nil {
			return nil, 0, "", err
		}
	}
	if size < 0 || size > 100 {
		return nil, 0, "", status.Error(codes.InvalidArgument, "page_size must be 0 to 100")
	}
	if size == 0 {
		size = 20
	}
	offset := 0
	if token != "" {
		if len(token) > 10 {
			return nil, 0, "", status.Error(codes.InvalidArgument, "invalid page token")
		}
		offset, err = strconv.Atoi(token)
		if err != nil || offset < 0 || offset > 2147483547 || strconv.Itoa(offset) != token {
			return nil, 0, "", status.Error(codes.InvalidArgument, "invalid page token")
		}
	}
	f := ports.Filter{JobID: jobID, Limit: int(size), Offset: offset}
	if i.Role == "applicant" {
		f.ApplicantID = i.UserID
	}
	if i.Role == "employer" {
		f.EmployerID = i.UserID
	}
	rows, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, "", mapped(err)
	}
	next := ""
	if int64(offset+len(rows)) < total && len(rows) > 0 {
		next = strconv.Itoa(offset + len(rows))
	}
	return rows, total, next, nil
}
