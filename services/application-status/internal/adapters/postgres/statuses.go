package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

const statusColumns = "application_id, applicant_id, employer_id, status, created_at, updated_at"

// Statuses stores application statuses and their history in the
// application_status database. The adapter never owns the pool.
type Statuses struct{ pool *pgxpool.Pool }

var _ ports.StatusRepository = (*Statuses)(nil)

func NewStatuses(pool *pgxpool.Pool) *Statuses { return &Statuses{pool: pool} }

// Create inserts the status and its first history entry in one transaction.
func (r *Statuses) Create(ctx context.Context, s domain.ApplicationStatus, first domain.HistoryEntry) error {
	if err := checkStatuses(s.Status, first.NewStatus); err != nil {
		return err
	}
	return r.inTx(ctx, "create application status failed", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO application_statuses (`+statusColumns+`)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (application_id) DO NOTHING`,
			s.ApplicationID, s.ApplicantID, s.EmployerID, s.Status.String(), s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrApplicationExists
		}
		if err := insertEntry(ctx, tx, first); err != nil {
			return err
		}
		return insertStatusEvent(ctx, tx, s, first)
	})
}

func (r *Statuses) Get(ctx context.Context, applicationID string) (domain.ApplicationStatus, error) {
	var s domain.ApplicationStatus
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT `+statusColumns+` FROM application_statuses WHERE application_id = $1`, applicationID).
		Scan(&s.ApplicationID, &s.ApplicantID, &s.EmployerID, &status, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationStatus{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.ApplicationStatus{}, hideDriverError(ctx, "get application status failed")
	}
	parsed, ok := domain.ParseStatus(status)
	if !ok {
		return domain.ApplicationStatus{}, errors.New("stored application status is not valid")
	}
	s.Status = parsed
	// Store and return UTC, as AGENTS.md Section 9 requires.
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s, nil
}

// Transition is one conditional UPDATE plus the history insert in one
// transaction, so of two concurrent transitions only one is stored and the
// history always matches the status.
func (r *Statuses) Transition(ctx context.Context, s domain.ApplicationStatus, expected domain.Status, entry domain.HistoryEntry) error {
	if err := checkStatuses(s.Status, entry.NewStatus); err != nil {
		return err
	}
	return r.inTx(ctx, "update application status failed", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE application_statuses SET status = $3, updated_at = $4
			 WHERE application_id = $1 AND status = $2`,
			s.ApplicationID, expected.String(), s.Status.String(), s.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM application_statuses WHERE application_id = $1)`,
				s.ApplicationID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrApplicationNotFound
			}
			return domain.ErrStatusChanged
		}
		if err := insertEntry(ctx, tx, entry); err != nil {
			return err
		}
		return insertStatusEvent(ctx, tx, s, entry)
	})
}

// ListHistory runs the count and the page in one read-only snapshot, so
// total_count matches the rows returned.
func (r *Statuses) ListHistory(ctx context.Context, applicationID string, page domain.Page) (entries []domain.HistoryEntry, total int, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list status history failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `SELECT count(*) FROM status_history WHERE application_id = $1`, applicationID).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "count status history failed")
	}
	rows, err := tx.Query(ctx,
		`SELECT application_id, previous_status, new_status, occurred_at FROM status_history
		 WHERE application_id = $1 ORDER BY occurred_at, history_id LIMIT $2 OFFSET $3`,
		applicationID, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list status history failed")
	}
	defer rows.Close()
	entries = []domain.HistoryEntry{}
	for rows.Next() {
		var e domain.HistoryEntry
		var previous *string
		var next string
		if err := rows.Scan(&e.ApplicationID, &previous, &next, &e.OccurredAt); err != nil {
			return nil, 0, hideDriverError(ctx, "list status history failed")
		}
		if previous != nil {
			if e.PreviousStatus, err = parseStored(*previous); err != nil {
				return nil, 0, err
			}
		}
		if e.NewStatus, err = parseStored(next); err != nil {
			return nil, 0, err
		}
		e.OccurredAt = e.OccurredAt.UTC()
		entries = append(entries, e)
	}
	if rows.Err() != nil {
		return nil, 0, hideDriverError(ctx, "list status history failed")
	}
	return entries, total, nil
}

// insertEntry stores UNSPECIFIED as NULL, which only the first entry may have.
func insertEntry(ctx context.Context, tx pgx.Tx, e domain.HistoryEntry) error {
	var previous *string
	if e.PreviousStatus != domain.StatusUnspecified {
		p := e.PreviousStatus.String()
		previous = &p
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO status_history (application_id, previous_status, new_status, occurred_at)
		 VALUES ($1, $2, $3, $4)`,
		e.ApplicationID, previous, e.NewStatus.String(), e.OccurredAt)
	return err
}

// inTx commits only if fn succeeds. Domain errors pass through; driver errors
// get a fixed message because they can contain SQL literals.
func (r *Statuses) inTx(ctx context.Context, message string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return hideDriverError(ctx, message)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		if errors.Is(err, domain.ErrApplicationExists) || errors.Is(err, domain.ErrApplicationNotFound) ||
			errors.Is(err, domain.ErrStatusChanged) || errors.Is(err, domain.ErrConflictingEvent) || errors.Is(err, domain.ErrInvalidTransition) {
			return err
		}
		return hideDriverError(ctx, message)
	}
	if err := tx.Commit(ctx); err != nil {
		return hideDriverError(ctx, message)
	}
	return nil
}

func checkStatuses(statuses ...domain.Status) error {
	for _, s := range statuses {
		if _, ok := domain.ParseStatus(s.String()); !ok {
			return errors.New("application status is not valid")
		}
	}
	return nil
}

func parseStored(s string) (domain.Status, error) {
	parsed, ok := domain.ParseStatus(s)
	if !ok {
		return domain.StatusUnspecified, errors.New("stored application status is not valid")
	}
	return parsed, nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
