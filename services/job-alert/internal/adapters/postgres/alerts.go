package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
)

type Alerts struct{ pool *pgxpool.Pool }

var _ ports.AlertRepository = (*Alerts)(nil)

func NewAlerts(pool *pgxpool.Pool) *Alerts { return &Alerts{pool} }

var errDatabase = errors.New("subscription database operation failed")

func databaseError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errDatabase
}
func (r *Alerts) Create(ctx context.Context, a domain.Alert) (domain.Alert, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Alert{}, databaseError(err)
	}
	defer tx.Rollback(ctx)
	// Serialize subscriptions for the same applicant, including when none exist yet.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.ApplicantID); err != nil {
		return domain.Alert{}, databaseError(err)
	}
	var duplicate bool
	var count int
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_alerts WHERE applicant_id=$1 AND category_ids=$2 AND tag_ids=$3 AND location=$4)`, a.ApplicantID, a.CategoryIDs, a.TagIDs, a.Location).Scan(&duplicate); err != nil {
		return domain.Alert{}, databaseError(err)
	}
	if duplicate {
		return domain.Alert{}, domain.ErrExists
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM job_alerts WHERE applicant_id=$1`, a.ApplicantID).Scan(&count); err != nil {
		return domain.Alert{}, databaseError(err)
	}
	if count >= 10 {
		return domain.Alert{}, domain.ErrLimit
	}
	names, err := json.Marshal(a.Names)
	if err != nil {
		return domain.Alert{}, databaseError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_alerts (alert_id,applicant_id,category_ids,tag_ids,location,names,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, a.ID, a.ApplicantID, a.CategoryIDs, a.TagIDs, a.Location, names, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return domain.Alert{}, databaseError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Alert{}, databaseError(err)
	}
	return a, nil
}
func (r *Alerts) List(ctx context.Context, owner string, p domain.Page) ([]domain.Alert, int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, databaseError(err)
	}
	defer tx.Rollback(ctx)
	var total int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM job_alerts WHERE applicant_id=$1`, owner).Scan(&total); err != nil {
		return nil, 0, databaseError(err)
	}
	rows, err := tx.Query(ctx, `SELECT alert_id,applicant_id,category_ids,tag_ids,location,names,created_at,updated_at FROM job_alerts WHERE applicant_id=$1 ORDER BY created_at,alert_id LIMIT $2 OFFSET $3`, owner, p.Size, p.Offset)
	if err != nil {
		return nil, 0, databaseError(err)
	}
	defer rows.Close()
	alerts := []domain.Alert{}
	for rows.Next() {
		var a domain.Alert
		var names []byte
		if err = rows.Scan(&a.ID, &a.ApplicantID, &a.CategoryIDs, &a.TagIDs, &a.Location, &names, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, databaseError(err)
		}
		if err = json.Unmarshal(names, &a.Names); err != nil {
			return nil, 0, databaseError(err)
		}
		a.CreatedAt = a.CreatedAt.UTC()
		a.UpdatedAt = a.UpdatedAt.UTC()
		alerts = append(alerts, a)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, databaseError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, 0, databaseError(err)
	}
	return alerts, total, nil
}
func (r *Alerts) Delete(ctx context.Context, id, owner string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM job_alerts WHERE alert_id=$1 AND applicant_id=$2`, id, owner)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var exists bool
	if err = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_alerts WHERE alert_id=$1)`, id).Scan(&exists); err != nil {
		return databaseError(err)
	}
	if exists {
		return domain.ErrPermissionDenied
	}
	return domain.ErrNotFound
}
