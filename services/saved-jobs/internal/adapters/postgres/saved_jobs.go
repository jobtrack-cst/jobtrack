package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

type SavedJobsRepository struct {
	pool *pgxpool.Pool
}

func NewSavedJobsRepository(pool *pgxpool.Pool) *SavedJobsRepository {
	return &SavedJobsRepository{pool: pool}
}

// lockJobSQL serialises SaveJob and CloseJob for one job, so a bookmark saved
// while job.closed is applied is either deleted or refused (JT-70). The ::uuid
// cast gives the same key for upper and lower case IDs.
const lockJobSQL = `SELECT pg_advisory_xact_lock(hashtextextended('saved-jobs:job:' || $1::uuid::text, 0))`

func (r *SavedJobsRepository) SaveJob(ctx context.Context, jobID, applicantID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, lockJobSQL, jobID); err != nil {
		return err
	}
	var isClosed bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM closed_jobs WHERE job_id = $1)", jobID).Scan(&isClosed); err != nil {
		return err
	}
	if isClosed {
		return domain.ErrJobClosed
	}
	if _, err := tx.Exec(ctx, "INSERT INTO saved_jobs (job_id, applicant_id) VALUES ($1, $2)", jobID, applicantID); err != nil {
		// pgx code 23505 is unique_violation
		if isUniqueViolation(err) {
			return domain.ErrAlreadySaved
		}
		return err
	}
	return tx.Commit(ctx)
}

// CloseJob claims the event ID, records the job as closed and deletes its
// bookmarks in one transaction; any failure rolls back all three.
func (r *SavedJobsRepository) CloseJob(ctx context.Context, closure domain.JobClosure) (domain.ClosureOutcome, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ClosureOutcome{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	claimed, err := tx.Exec(ctx, "INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING", closure.EventID)
	if err != nil {
		return domain.ClosureOutcome{}, err
	}
	if claimed.RowsAffected() == 0 {
		return domain.ClosureOutcome{Duplicate: true}, nil
	}
	if _, err := tx.Exec(ctx, lockJobSQL, closure.JobID); err != nil {
		return domain.ClosureOutcome{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO closed_jobs (job_id, closed_at) VALUES ($1, $2) ON CONFLICT (job_id) DO NOTHING",
		closure.JobID, closure.ClosedAt); err != nil {
		return domain.ClosureOutcome{}, err
	}
	deleted, err := tx.Exec(ctx, "DELETE FROM saved_jobs WHERE job_id = $1", closure.JobID)
	if err != nil {
		return domain.ClosureOutcome{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ClosureOutcome{}, err
	}
	return domain.ClosureOutcome{RemovedBookmarks: int(deleted.RowsAffected())}, nil
}

func (r *SavedJobsRepository) UnsaveJob(ctx context.Context, jobID, applicantID string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM saved_jobs WHERE job_id = $1 AND applicant_id = $2", jobID, applicantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotSaved
	}
	return nil
}

func (r *SavedJobsRepository) ListMySavedJobs(ctx context.Context, applicantID string, limit, offset int) (domain.Page, error) {
	var total int
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM saved_jobs WHERE applicant_id = $1", applicantID).Scan(&total)
	if err != nil {
		return domain.Page{}, err
	}

	rows, err := r.pool.Query(ctx, `
        SELECT job_id, applicant_id, saved_at
        FROM saved_jobs
        WHERE applicant_id = $1
        ORDER BY saved_at DESC
        LIMIT $2 OFFSET $3`, applicantID, limit, offset)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()

	var jobs []domain.SavedJob
	for rows.Next() {
		var j domain.SavedJob
		if err := rows.Scan(&j.JobID, &j.ApplicantID, &j.SavedAt); err != nil {
			return domain.Page{}, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}

	return domain.Page{Jobs: jobs, Total: total}, nil
}

func (r *SavedJobsRepository) ListJobSavers(ctx context.Context, jobID string, limit, offset int) ([]string, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM saved_jobs WHERE job_id = $1", jobID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
        SELECT applicant_id
        FROM saved_jobs
        WHERE job_id = $1
        ORDER BY saved_at DESC
        LIMIT $2 OFFSET $3`, jobID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var applicants []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		applicants = append(applicants, id)
	}
	return applicants, total, rows.Err()
}

// isUniqueViolation checks for PostgreSQL error code 23505
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
