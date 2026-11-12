package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = "application_id, applicant_id, job_id, resume_id, employer_id, withdrawn, created_at, updated_at"

func scan(row pgx.Row) (domain.Application, error) {
	var a domain.Application
	err := row.Scan(&a.ID, &a.ApplicantID, &a.JobID, &a.ResumeID, &a.EmployerID, &a.Withdrawn, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, domain.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return a, domain.ErrDuplicate
	}
	if err == nil {
		a.CreatedAt = a.CreatedAt.UTC()
		a.UpdatedAt = a.UpdatedAt.UTC()
	}
	return a, err
}
func (r *Repository) Create(ctx context.Context, a domain.Application) (domain.Application, error) {
	return r.write(ctx, false, func(tx pgx.Tx) (domain.Application, error) {
		return scan(tx.QueryRow(ctx, "INSERT INTO applications ("+columns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING "+columns, a.ID, a.ApplicantID, a.JobID, a.ResumeID, a.EmployerID, a.Withdrawn, a.CreatedAt, a.UpdatedAt))
	})
}
func (r *Repository) Get(ctx context.Context, id string) (domain.Application, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT "+columns+" FROM applications WHERE application_id=$1", id))
}
func (r *Repository) Withdraw(ctx context.Context, id, owner string) (domain.Application, error) {
	return r.write(ctx, true, func(tx pgx.Tx) (domain.Application, error) {
		a, err := scan(tx.QueryRow(ctx, "UPDATE applications SET withdrawn=true, updated_at=clock_timestamp() WHERE application_id=$1 AND applicant_id=$2 AND NOT withdrawn RETURNING "+columns, id, owner))
		if errors.Is(err, domain.ErrNotFound) {
			return a, domain.ErrWithdrawn
		}
		return a, err
	})
}
func (r *Repository) List(ctx context.Context, f ports.Filter) ([]domain.Application, int64, error) {
	// A repeatable snapshot keeps total and page consistent during concurrent submissions.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	const where = " FROM applications WHERE ($1='' OR applicant_id::text=$1) AND ($2='' OR employer_id::text=$2) AND ($3='' OR job_id::text=$3)"
	var total int64
	if err = tx.QueryRow(ctx, "SELECT count(*)"+where, f.ApplicantID, f.EmployerID, f.JobID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, "SELECT "+columns+where+" ORDER BY created_at DESC, application_id DESC LIMIT $4 OFFSET $5", f.ApplicantID, f.EmployerID, f.JobID, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []domain.Application{}
	for rows.Next() {
		a, e := scan(rows)
		if e != nil {
			return nil, 0, e
		}
		result = append(result, a)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}
