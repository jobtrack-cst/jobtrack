package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/saved_jobs_mock.go -package=mocks . Repository

type Repository interface {
	// SaveJob returns ErrAlreadySaved if the record exists.
	// It returns ErrJobClosed if the job is in the closed_jobs table.
	SaveJob(ctx context.Context, jobID, applicantID string) error

	// UnsaveJob returns ErrNotSaved if the record does not exist.
	UnsaveJob(ctx context.Context, jobID, applicantID string) error

	// ListMySavedJobs returns the applicant's saved jobs, newest first.
	ListMySavedJobs(ctx context.Context, applicantID string, limit, offset int) (domain.Page, error)

	// ListJobSavers returns the applicant IDs who saved a job.
	ListJobSavers(ctx context.Context, jobID string, limit, offset int) ([]string, int, error)

	// CloseJob applies a job.closed event in one transaction: it claims the
	// event ID in processed_events, records the job in closed_jobs and deletes
	// every bookmark for it. An already processed event ID changes nothing.
	CloseJob(ctx context.Context, closure domain.JobClosure) (domain.ClosureOutcome, error)
}
