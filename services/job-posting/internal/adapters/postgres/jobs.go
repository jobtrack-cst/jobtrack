package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
)

const jobColumns = "job_id, employer_id, title, description, category_id, tag_ids, location, deadline, status, created_at, updated_at"

// Jobs stores jobs in the job_posting database. The adapter never owns the pool.
type Jobs struct{ pool *pgxpool.Pool }

var _ ports.JobRepository = (*Jobs)(nil)

func NewJobs(pool *pgxpool.Pool) *Jobs { return &Jobs{pool: pool} }

func (r *Jobs) Create(ctx context.Context, j domain.Job) (domain.Job, error) {
	if _, ok := domain.ParseStatus(j.Status.String()); !ok {
		return domain.Job{}, errors.New("job status is not valid")
	}
	created, err := scanJob(r.pool.QueryRow(ctx,
		`INSERT INTO jobs (employer_id, title, description, category_id, tag_ids, location,
		     deadline, status, created_at, updated_at)
		 VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5::uuid[], $6, $7, $8, $9, $10)
		 RETURNING `+jobColumns,
		j.EmployerID, j.Title, j.Description, j.CategoryID, tagIDsParam(j.TagIDs), j.Location,
		j.Deadline, j.Status.String(), j.CreatedAt, j.UpdatedAt))
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.Job{}, hideDriverError(ctx, "create job failed")
	}
	return created, nil
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

// UpdateFields never writes status, employer_id or created_at, and only
// updates a row still in the expected status.
func (r *Jobs) UpdateFields(ctx context.Context, j domain.Job, expected domain.Status) (domain.Job, error) {
	updated, err := scanJob(r.pool.QueryRow(ctx,
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
		return domain.Job{}, hideDriverError(ctx, "update job failed")
	}
	return updated, nil
}

// UpdateStatus is one conditional UPDATE, so of two concurrent requests only
// one can move the job out of the from status.
func (r *Jobs) UpdateStatus(ctx context.Context, jobID string, from, to domain.Status, updatedAt time.Time) (domain.Job, error) {
	if _, ok := domain.ParseStatus(to.String()); !ok {
		return domain.Job{}, errors.New("job status is not valid")
	}
	updated, err := scanJob(r.pool.QueryRow(ctx,
		`UPDATE jobs SET status = $3, updated_at = $4
		 WHERE job_id = $1 AND status = $2
		 RETURNING `+jobColumns,
		jobID, from.String(), to.String(), updatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Job{}, r.missOrConflict(ctx, jobID)
	}
	if err != nil {
		return domain.Job{}, hideDriverError(ctx, "update job status failed")
	}
	return updated, nil
}

// missOrConflict tells a missing job apart from one whose status changed.
func (r *Jobs) missOrConflict(ctx context.Context, jobID string) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM jobs WHERE job_id = $1)`, jobID).Scan(&exists); err != nil {
		return hideDriverError(ctx, "check job failed")
	}
	if !exists {
		return domain.ErrJobNotFound
	}
	return domain.ErrJobStatusChanged
}

func (r *Jobs) ListByEmployer(ctx context.Context, employerID string, status domain.Status, page domain.Page) ([]domain.Job, int, error) {
	// An empty status matches every status (StatusUnspecified means all).
	return r.list(ctx,
		`FROM jobs WHERE employer_id = $1 AND ($2 = '' OR status = $2)`,
		`ORDER BY created_at DESC, job_id DESC`,
		page, employerID, status.String())
}

func (r *Jobs) ListClosingBetween(ctx context.Context, w domain.ClosingWindow, page domain.Page) ([]domain.Job, int, error) {
	return r.list(ctx,
		`FROM jobs WHERE status = 'published' AND deadline >= $1 AND deadline < $2`,
		`ORDER BY deadline, job_id`,
		page, w.Start, w.End)
}

// list runs the count and the page in one read-only snapshot, so total_count
// matches the rows returned.
func (r *Jobs) list(ctx context.Context, from, order string, page domain.Page, args ...any) (jobs []domain.Job, total int, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list jobs failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "count jobs failed")
	}
	n := len(args)
	query := `SELECT ` + jobColumns + ` ` + from + ` ` + order +
		` LIMIT $` + strconv.Itoa(n+1) + ` OFFSET $` + strconv.Itoa(n+2)
	rows, err := tx.Query(ctx, query, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list jobs failed")
	}
	defer rows.Close()
	jobs = []domain.Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, 0, hideDriverError(ctx, "list jobs failed")
		}
		jobs = append(jobs, job)
	}
	if rows.Err() != nil {
		return nil, 0, hideDriverError(ctx, "list jobs failed")
	}
	return jobs, total, nil
}

func scanJob(row pgx.Row) (domain.Job, error) {
	var j domain.Job
	var categoryID *string
	var status string
	if err := row.Scan(&j.ID, &j.EmployerID, &j.Title, &j.Description, &categoryID, &j.TagIDs,
		&j.Location, &j.Deadline, &status, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return domain.Job{}, err
	}
	parsed, ok := domain.ParseStatus(status)
	if !ok {
		return domain.Job{}, errors.New("stored job status is not valid")
	}
	j.Status = parsed
	if categoryID != nil {
		j.CategoryID = *categoryID
	}
	if j.TagIDs == nil {
		j.TagIDs = []string{}
	}
	// Store and return UTC, as AGENTS.md Section 9 requires.
	j.Deadline, j.CreatedAt, j.UpdatedAt = j.Deadline.UTC(), j.CreatedAt.UTC(), j.UpdatedAt.UTC()
	return j, nil
}

// tagIDsParam sends an empty array instead of NULL for a nil slice.
func tagIDsParam(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
