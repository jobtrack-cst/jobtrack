package ports

//go:generate go tool mockgen -destination=mocks/reports_mock.go -package=mocks . ReportRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

// ReportRepository stores reports and the jobs moderation has removed.
// Moderation has no allowed call to job-posting (AGENTS.md Section 6), so
// there are no client ports and it only knows the jobs it removed itself.
// Publishing moderation.job_removed uses EventPublisher in Sprint 2.
type ReportRepository interface {
	// Create stores a new OPEN report. It holds a per-job lock shared with
	// RemoveJob, so a report can never slip in as OPEN beside a removal. It
	// returns domain.ErrJobAlreadyRemoved if moderation removed the job, or
	// domain.ErrOpenReportExists if the reporter already has an OPEN report
	// for the job, including when two reports race. The removed check wins.
	Create(ctx context.Context, r domain.Report) (domain.Report, error)
	// GetByID returns domain.ErrReportNotFound for an unknown report.
	GetByID(ctx context.Context, id string) (domain.Report, error)
	// List returns one page of reports, newest first with report_id breaking
	// ties, and the total count. domain.StatusUnspecified means all statuses.
	List(ctx context.Context, status domain.ReportStatus, page domain.Page) ([]domain.Report, int64, error)
	// Dismiss writes status and updated_at only while the stored row is still
	// OPEN, so two admins cannot both decide. It returns
	// domain.ErrReportNotOpen if the row was already closed, or
	// domain.ErrReportNotFound if it does not exist.
	Dismiss(ctx context.Context, r domain.Report) (domain.Report, error)
	// RemoveJob records the removal and marks every OPEN report for the job as
	// ACTIONED (updated_at = RemovedAt) in one transaction under the per-job
	// lock. It returns domain.ErrJobAlreadyRemoved if the job was removed
	// before, including when two admins race.
	RemoveJob(ctx context.Context, rm domain.Removal) error
}

// Clock makes created_at, updated_at and removed_at testable.
type Clock interface {
	Now() time.Time
}
