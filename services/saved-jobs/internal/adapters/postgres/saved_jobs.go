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

func (r *SavedJobsRepository) SaveJob(ctx context.Context, jobID, applicantID string) error {
	// First check if the job is closed
	var isClosed bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM closed_jobs WHERE job_id = $1)", jobID).Scan(&isClosed)
	if err != nil {
		return err
	}
	if isClosed {
		return domain.ErrJobClosed
	}

	// Try to insert
	_, err = r.pool.Exec(ctx, "INSERT INTO saved_jobs (job_id, applicant_id) VALUES ($1, $2)", jobID, applicantID)
	if err != nil {
		// pgx code 23505 is unique_violation
		if isUniqueViolation(err) {
			return domain.ErrAlreadySaved
		}
		return err
	}
	return nil
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
