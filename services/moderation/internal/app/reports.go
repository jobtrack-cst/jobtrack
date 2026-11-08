package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports"
)

// Reports holds the moderation use cases. The gRPC adapter builds the Caller
// from pkg/authctx and translates domain errors to gRPC codes. Every use case
// checks the role first, then the input, then stored state, so a caller with
// the wrong role learns nothing else.
type Reports struct {
	repo  ports.ReportRepository
	clock ports.Clock
}

func NewReports(repo ports.ReportRepository, clock ports.Clock) *Reports {
	return &Reports{repo: repo, clock: clock}
}

// ListResult is one page of reports with the values the proto pagination needs.
type ListResult struct {
	Reports       []domain.Report
	TotalCount    int64
	NextPageToken string
}

// Report creates an OPEN report. Any authenticated role may report, and the
// reporter ID always comes from the caller, never from the input. The
// repository rejects removed jobs and duplicate OPEN reports atomically.
func (s *Reports) Report(ctx context.Context, caller domain.Caller, jobID, reason string) (domain.Report, error) {
	job, err := domain.NormalizeID(jobID, domain.ErrInvalidJobID)
	if err != nil {
		return domain.Report{}, err
	}
	text, err := domain.NewReportReason(reason)
	if err != nil {
		return domain.Report{}, err
	}
	r := domain.NewReport(uuid.NewString(), job, caller.UserID, text, s.clock.Now())
	return s.repo.Create(ctx, r)
}

// List returns reports for the admin, newest first. The status stays a proto
// enum number until after the role check, so a non-admin never learns
// whether a filter is valid.
func (s *Reports) List(ctx context.Context, caller domain.Caller, status int32, pageSize int32, pageToken string) (ListResult, error) {
	if err := caller.RequireAdmin(); err != nil {
		return ListResult{}, err
	}
	filter, err := domain.ParseStatusFilter(status)
	if err != nil {
		return ListResult{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return ListResult{}, err
	}
	items, total, err := s.repo.List(ctx, filter, page)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Reports: items, TotalCount: total, NextPageToken: page.NextToken(len(items), total)}, nil
}

// Dismiss closes an OPEN report with no action. The domain check gives a
// clear error for a report already closed, and the repository's conditional
// update covers two admins racing.
func (s *Reports) Dismiss(ctx context.Context, caller domain.Caller, reportID string) (domain.Report, error) {
	if err := caller.RequireAdmin(); err != nil {
		return domain.Report{}, err
	}
	id, err := domain.NormalizeID(reportID, domain.ErrInvalidReportID)
	if err != nil {
		return domain.Report{}, err
	}
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Report{}, err
	}
	if err := r.Dismiss(s.clock.Now()); err != nil {
		return domain.Report{}, err
	}
	return s.repo.Dismiss(ctx, r)
}

// RemoveJob takes a job down and returns its normalized ID. A job with no
// reports can still be removed. Publishing moderation.job_removed is Sprint 2.
func (s *Reports) RemoveJob(ctx context.Context, caller domain.Caller, jobID, reason string) (string, error) {
	if err := caller.RequireAdmin(); err != nil {
		return "", err
	}
	job, err := domain.NormalizeID(jobID, domain.ErrInvalidJobID)
	if err != nil {
		return "", err
	}
	text, err := domain.NewRemovalReason(reason)
	if err != nil {
		return "", err
	}
	if err := s.repo.RemoveJob(ctx, domain.NewRemoval(job, caller.UserID, text, s.clock.Now())); err != nil {
		return "", err
	}
	return job, nil
}
