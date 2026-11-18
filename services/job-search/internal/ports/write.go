package ports

//go:generate go tool mockgen -destination=mocks/write_mock.go -package=mocks . WriteRepository

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

// WriteRepository updates the job read model from events.
type WriteRepository interface {
	// UpsertJob creates or updates a job. It ignores the update if the
	// existing row has a newer updated_at timestamp.
	UpsertJob(ctx context.Context, job domain.JobSummary, updatedAt time.Time) error

	// CloseJob marks a job as closed. It ignores the update if the
	// existing row has a newer updated_at timestamp.
	CloseJob(ctx context.Context, jobID string, updatedAt time.Time) error

	// IsProcessed checks if the event_id has already been processed.
	IsProcessed(ctx context.Context, eventID string) (bool, error)

	// MarkProcessed records an event_id as processed.
	MarkProcessed(ctx context.Context, eventID string) error
}
