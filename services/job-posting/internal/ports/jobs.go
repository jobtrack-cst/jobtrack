package ports

//go:generate go tool mockgen -destination=mocks/jobs_mock.go -package=mocks . JobRepository,EmployerVerifier,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
)

// JobRepository stores jobs. The database generates job IDs.
type JobRepository interface {
	// Create stores a new job and returns the stored row with its ID.
	Create(ctx context.Context, job domain.Job) (domain.Job, error)
	// GetByID returns domain.ErrJobNotFound when there is no such job.
	GetByID(ctx context.Context, jobID string) (domain.Job, error)
	// UpdateFields writes the editable fields and updated_at only if the
	// stored status still equals expected. It returns domain.ErrJobNotFound,
	// or domain.ErrJobStatusChanged after a concurrent status change.
	UpdateFields(ctx context.Context, job domain.Job, expected domain.Status) (domain.Job, error)
	// UpdateStatus moves a job from one status to another in one conditional
	// update. It returns domain.ErrJobNotFound, or domain.ErrJobStatusChanged
	// if the job is no longer in the from status.
	UpdateStatus(ctx context.Context, jobID string, from, to domain.Status, updatedAt time.Time) (domain.Job, error)
	// ListByEmployer lists one employer's jobs, newest first (job_id breaks
	// ties). StatusUnspecified means all statuses. It also returns the total count.
	ListByEmployer(ctx context.Context, employerID string, status domain.Status, page domain.Page) ([]domain.Job, int, error)
	// ListClosingBetween lists PUBLISHED jobs with Start <= deadline < End,
	// ordered by deadline then job_id. It also returns the total count.
	ListClosingBetween(ctx context.Context, window domain.ClosingWindow, page domain.Page) ([]domain.Job, int, error)
}

// EmployerVerifier checks whether an employer's company profile is VERIFIED.
// Sprint 1 and 2: mocked with gomock. JT-57: real client calling
// EmployerService.GetCompanyProfile with the user identity forwarded.
// An employer without a profile is not verified (false, nil).
type EmployerVerifier interface {
	IsVerified(ctx context.Context, employerID string) (bool, error)
}

// Clock makes created_at, updated_at and deadline checks testable.
type Clock interface {
	Now() time.Time
}
