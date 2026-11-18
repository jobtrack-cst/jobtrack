package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

type WriteRepository struct {
	pool *pgxpool.Pool
}

func NewWriteRepository(pool *pgxpool.Pool) *WriteRepository {
	return &WriteRepository{pool: pool}
}

func (r *WriteRepository) UpsertJob(ctx context.Context, job domain.JobSummary, updatedAt time.Time) error {
	// If a row already exists, only update if the new event is NEWER.
	// COALESCE ensures nil tag arrays don't violate the NOT NULL constraint.
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jobs (job_id, employer_id, title, category_id, tag_ids, location, deadline, closed, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, COALESCE($5::text[], '{}'), $6, $7, FALSE, $8)
		ON CONFLICT (job_id) DO UPDATE SET
			employer_id = EXCLUDED.employer_id,
			title = EXCLUDED.title,
			category_id = EXCLUDED.category_id,
			tag_ids = EXCLUDED.tag_ids,
			location = EXCLUDED.location,
			deadline = EXCLUDED.deadline,
			closed = FALSE,
			updated_at = EXCLUDED.updated_at
		WHERE EXCLUDED.updated_at > jobs.updated_at`,
		job.JobID, job.EmployerID, job.Title, job.CategoryID, job.TagIDs, job.Location, job.Deadline, updatedAt)
	return err
}

func (r *WriteRepository) CloseJob(ctx context.Context, jobID string, updatedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET closed = TRUE, updated_at = $2
		WHERE job_id = $1 AND updated_at < $2`,
		jobID, updatedAt)
	return err
}

func (r *WriteRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM processed_event_id WHERE event_id = $1)", eventID).Scan(&exists)
	return exists, err
}

func (r *WriteRepository) MarkProcessed(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO processed_event_id (event_id) VALUES ($1) ON CONFLICT DO NOTHING", eventID)
	return err
}
