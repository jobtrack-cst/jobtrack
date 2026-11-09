package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports"
)

const (
	uniqueViolation = "23505"
	oneOpenIndex    = "reports_one_open_per_reporter"
)

const reportColumns = "id, job_id, reporter_id, reason, status, created_at, updated_at"

// lockJob serializes Create and RemoveJob for one job until the transaction
// ends. Without it a report could pass the removed check, then commit as OPEN
// after a removal had already marked the job's reports ACTIONED. Two job IDs
// sharing a hash only wait for each other, which is harmless.
const lockJob = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`

// Reports stores reports and removed jobs in the moderation database. The
// adapter never owns the pool.
type Reports struct{ pool *pgxpool.Pool }

var _ ports.ReportRepository = (*Reports)(nil)

func NewReports(pool *pgxpool.Pool) *Reports { return &Reports{pool: pool} }

// Create runs at READ COMMITTED (the default), so after waiting for the lock
// each statement sees a removal that committed in the meantime.
func (r *Reports) Create(ctx context.Context, v domain.Report) (domain.Report, error) {
	status, ok := statusText(v.Status)
	if !ok || v.Status != domain.StatusOpen {
		return domain.Report{}, errors.New("new report status is not valid")
	}
	var created domain.Report
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockJob, v.JobID); err != nil {
			return err
		}
		var removed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM removed_jobs WHERE job_id = $1)`, v.JobID).Scan(&removed); err != nil {
			return err
		}
		if removed {
			return domain.ErrJobAlreadyRemoved
		}
		var err error
		created, err = scanReport(tx.QueryRow(ctx,
			`INSERT INTO reports (id, job_id, reporter_id, reason, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING `+reportColumns,
			v.ID, v.JobID, v.ReporterID, v.Reason, status, v.CreatedAt, v.UpdatedAt))
		return err
	})
	if errors.Is(err, domain.ErrJobAlreadyRemoved) {
		return domain.Report{}, err
	}
	if mapped := uniqueError(err); mapped != nil {
		return domain.Report{}, mapped
	}
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.Report{}, safe(ctx, "create report failed")
	}
	return created, nil
}

func (r *Reports) GetByID(ctx context.Context, id string) (domain.Report, error) {
	v, err := scanReport(r.pool.QueryRow(ctx,
		`SELECT `+reportColumns+` FROM reports WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrReportNotFound
	}
	if err != nil {
		return domain.Report{}, safe(ctx, "get report failed")
	}
	return v, nil
}

func (r *Reports) List(ctx context.Context, status domain.ReportStatus, page domain.Page) ([]domain.Report, int64, error) {
	const order = `ORDER BY created_at DESC, id DESC`
	if status == domain.StatusUnspecified {
		return r.list(ctx, ``, order, page)
	}
	name, ok := statusText(status)
	if !ok {
		return nil, 0, errors.New("status filter is not valid")
	}
	return r.list(ctx, `WHERE status = $1`, order, page, name)
}

// list runs the count and the page in one read-only snapshot, so total_count
// matches the rows returned. where and order are fixed strings from this file.
func (r *Reports) list(ctx context.Context, where, order string, page domain.Page, args ...any) ([]domain.Report, int64, error) {
	var items []domain.Report
	var total int64
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM reports `+where, args...).Scan(&total); err != nil {
			return err
		}
		n := len(args)
		rows, err := tx.Query(ctx,
			`SELECT `+reportColumns+` FROM reports `+where+` `+order+
				` LIMIT $`+strconv.Itoa(n+1)+` OFFSET $`+strconv.Itoa(n+2),
			append(args, page.Limit, page.Offset)...)
		if err != nil {
			return err
		}
		items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Report, error) {
			return scanReport(row)
		})
		return err
	})
	if err != nil {
		return nil, 0, safe(ctx, "list reports failed")
	}
	return items, total, nil
}

// Dismiss only changes a row that is still OPEN, so two admins acting at once
// cannot both succeed, and a report actioned by RemoveJob stays actioned.
func (r *Reports) Dismiss(ctx context.Context, v domain.Report) (domain.Report, error) {
	if v.Status != domain.StatusDismissed {
		return domain.Report{}, errors.New("dismiss status is not valid")
	}
	dismissed, err := scanReport(r.pool.QueryRow(ctx,
		`UPDATE reports SET status = 'dismissed', updated_at = $2
		 WHERE id = $1 AND status = 'open'
		 RETURNING `+reportColumns,
		v.ID, v.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, r.whyNotDismissed(ctx, v.ID)
	}
	if err != nil {
		return domain.Report{}, safe(ctx, "dismiss report failed")
	}
	return dismissed, nil
}

// whyNotDismissed tells a missing row apart from one already closed.
func (r *Reports) whyNotDismissed(ctx context.Context, id string) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM reports WHERE id = $1)`, id).Scan(&exists); err != nil {
		return safe(ctx, "dismiss report failed")
	}
	if !exists {
		return domain.ErrReportNotFound
	}
	return domain.ErrReportNotOpen
}

// RemoveJob records the takedown and actions the job's OPEN reports together.
// ON CONFLICT keeps the transaction usable, and the primary key still stops a
// second takedown if two admins race.
func (r *Reports) RemoveJob(ctx context.Context, rm domain.Removal) error {
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockJob, rm.JobID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`INSERT INTO removed_jobs (job_id, reason, removed_by, removed_at)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (job_id) DO NOTHING`,
			rm.JobID, rm.Reason, rm.RemovedBy, rm.RemovedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrJobAlreadyRemoved
		}
		_, err = tx.Exec(ctx,
			`UPDATE reports SET status = 'actioned', updated_at = $2
			 WHERE job_id = $1 AND status = 'open'`,
			rm.JobID, rm.RemovedAt)
		return err
	})
	if errors.Is(err, domain.ErrJobAlreadyRemoved) {
		return err
	}
	if err != nil {
		return safe(ctx, "remove job failed")
	}
	return nil
}

func uniqueError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return nil
	}
	if pgErr.ConstraintName == oneOpenIndex {
		return domain.ErrOpenReportExists
	}
	return nil
}

func scanReport(row pgx.Row) (domain.Report, error) {
	var v domain.Report
	var status string
	if err := row.Scan(&v.ID, &v.JobID, &v.ReporterID, &v.Reason, &status, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return domain.Report{}, err
	}
	parsed, ok := parseStatus(status)
	if !ok {
		return domain.Report{}, errors.New("stored report status is not valid")
	}
	v.Status = parsed
	// Store and return UTC, as AGENTS.md Section 9 requires.
	v.CreatedAt, v.UpdatedAt = v.CreatedAt.UTC(), v.UpdatedAt.UTC()
	return v, nil
}

var statusNames = map[domain.ReportStatus]string{
	domain.StatusOpen:      "open",
	domain.StatusDismissed: "dismissed",
	domain.StatusActioned:  "actioned",
}

func statusText(s domain.ReportStatus) (string, bool) {
	name, ok := statusNames[s]
	return name, ok
}

func parseStatus(name string) (domain.ReportStatus, bool) {
	for s, n := range statusNames {
		if n == name {
			return s, true
		}
	}
	return domain.StatusUnspecified, false
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
