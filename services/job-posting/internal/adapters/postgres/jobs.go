package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
)

const jobColumns = "job_id, employer_id, title, description, coalesce(category_id::text, ''), coalesce(tag_ids::text[], '{}'), location, deadline, status, created_at, updated_at"

type Jobs struct {
	pool *pgxpool.Pool
}

func NewJobs(pool *pgxpool.Pool) *Jobs {
	return &Jobs{pool: pool}
}

func (r *Jobs) Create(ctx context.Context, j domain.Job) (domain.Job, error) {
	created, err := scanJob(r.pool.QueryRow(ctx,
		`INSERT INTO jobs (employer_id, title, description, category_id, tag_ids, location, deadline, status, created_at, updated_at)
		 VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5::uuid[], $6, $7, $8, $9, $10)
		 RETURNING `+jobColumns,
		j.EmployerID, j.Title, j.Description, j.CategoryID, tagIDsParam(j.TagIDs),
		j.Location, j.Deadline, j.Status.String(), j.CreatedAt, j.UpdatedAt))
	if err != nil {
		return domain.Job{}, hideDriverError(ctx, "create job failed")
	}
	return created, nil
}

func (r *Jobs) UpdateFields(ctx context.Context, j domain.Job, expected domain.Status) (domain.Job, error) {
	return r.write(ctx, "job.updated", func(tx pgx.Tx) (domain.Job, error) {
		updated, err := scanJob(tx.QueryRow(ctx,
			`UPDATE jobs
			 SET title = $3, description = $4, category_id = NULLIF($5, '')::uuid, tag_ids = $6::uuid[],
			     location = $7, deadline = $8, updated_at = $9
			 WHERE job_id = $1 AND status = $2
			 RETURNING `+jobColumns,
			j.ID, expected.String(), j.Title, j.Description, j.CategoryID, tagIDsParam(j.TagIDs),
			j.Location, j.Deadline, j.UpdatedAt))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Job{}, r.missOrConflict(ctx, j.ID)
		}
		if err != nil {
			if isLockTimeout(err) {
				return domain.Job{}, domain.ErrJobStatusChanged
			}
			return domain.Job{}, hideDriverError(ctx, "update job failed")
		}
		return updated, nil
	})
}

func (r *Jobs) UpdateStatus(ctx context.Context, jobID string, from, to domain.Status, updatedAt time.Time) (domain.Job, error) {
	if _, ok := domain.ParseStatus(to.String()); !ok {
		return domain.Job{}, errors.New("job status is not valid")
	}

	routingKey := ""
	if to == domain.StatusPublished {
		routingKey = "job.published"
	} else if to == domain.StatusClosed {
		routingKey = "job.closed"
	} else {
		return domain.Job{}, errors.New("unexpected status transition")
	}

	return r.write(ctx, routingKey, func(tx pgx.Tx) (domain.Job, error) {
		updated, err := scanJob(tx.QueryRow(ctx,
			`UPDATE jobs SET status = $3, updated_at = $4
			 WHERE job_id = $1 AND status = $2
			 RETURNING `+jobColumns,
			jobID, from.String(), to.String(), updatedAt))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Job{}, r.missOrConflict(ctx, jobID)
		}
		if err != nil {
			if isLockTimeout(err) {
				return domain.Job{}, domain.ErrJobStatusChanged
			}
			return domain.Job{}, hideDriverError(ctx, "update job status failed")
		}
		return updated, nil
	})
}

func (r *Jobs) GetByID(ctx context.Context, jobID string) (domain.Job, error) {
	job, err := scanJob(r.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE job_id = $1`, jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Job{}, domain.ErrJobNotFound
	}
	if err != nil {
		return domain.Job{}, hideDriverError(ctx, "get job failed")
	}
	return job, nil
}

func (r *Jobs) ListByEmployer(ctx context.Context, employerID string, status domain.Status, page domain.Page) ([]domain.Job, int, error) {
	where := `WHERE employer_id = $1`
	args := []any{employerID}
	if status != domain.StatusUnspecified {
		where += ` AND status = $2`
		args = append(args, status.String())
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM jobs `+where, args...).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "list jobs failed")
	}

	args = append(args, page.Limit, page.Offset)
	rows, err := r.pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs `+where+
		` ORDER BY created_at DESC, job_id DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list jobs failed")
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, hideDriverError(ctx, "list jobs failed")
		}
		out = append(out, j)
	}
	return out, total, rows.Err()
}

func (r *Jobs) ListClosingBetween(ctx context.Context, window domain.ClosingWindow, page domain.Page) ([]domain.Job, int, error) {
	where := `WHERE status = 'published' AND deadline >= $1 AND deadline < $2`
	args := []any{window.Start, window.End}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM jobs `+where, args...).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "list closing jobs failed")
	}

	args = append(args, page.Limit, page.Offset)
	rows, err := r.pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs `+where+
		` ORDER BY deadline ASC, job_id ASC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list closing jobs failed")
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, hideDriverError(ctx, "list closing jobs failed")
		}
		out = append(out, j)
	}
	return out, total, rows.Err()
}

// RecordJobRemoved implements the moderation consumer. It updates the job
// to REMOVED and stores the event ID in processed_events in a single transaction.
func (r *Jobs) RecordJobRemoved(ctx context.Context, eventID, jobID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	// 1. Check if already processed (idempotency)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`, eventID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil // Already processed, ignore
	}

	// 2. Mark as processed
	if _, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1)`, eventID); err != nil {
		return err
	}

	// 3. Update job status to REMOVED (only if it's not already removed)
	_, err = tx.Exec(ctx, `UPDATE jobs SET status = 'removed', updated_at = now() WHERE job_id = $1 AND status != 'REMOVED'`, jobID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Helpers
func scanJob(row pgx.Row) (domain.Job, error) {
	var j domain.Job
	var statusStr string
	err := row.Scan(
		&j.ID, &j.EmployerID, &j.Title, &j.Description, &j.CategoryID, &j.TagIDs,
		&j.Location, &j.Deadline, &statusStr, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return domain.Job{}, err
	}
	status, _ := domain.ParseStatus(statusStr)
	j.Status = status
	return j, nil
}

func (r *Jobs) missOrConflict(ctx context.Context, id string) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE job_id = $1)`, id).Scan(&exists)
	if err != nil {
		return hideDriverError(ctx, "check job failed")
	}
	if !exists {
		return domain.ErrJobNotFound
	}
	return domain.ErrJobStatusChanged
}

func tagIDsParam(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

func hideDriverError(_ context.Context, msg string) error {
	return errors.New(msg)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func isLockTimeout(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "55P03" // lock_not_available
	}
	return false
}
