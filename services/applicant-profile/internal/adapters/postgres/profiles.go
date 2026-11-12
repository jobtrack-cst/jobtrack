package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports"
)

// Profiles stores applicant profiles in the applicant_profile database. The
// adapter never owns the pool.
type Profiles struct{ pool *pgxpool.Pool }

var _ ports.ProfileRepository = (*Profiles)(nil)

func NewProfiles(pool *pgxpool.Pool) *Profiles { return &Profiles{pool: pool} }

// Create inserts the profile with its education and skills in one transaction.
func (r *Profiles) Create(ctx context.Context, p domain.Profile) error {
	return r.inTx(ctx, "create applicant profile failed", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO applicant_profiles (applicant_id, summary, created_at, updated_at)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (applicant_id) DO NOTHING`,
			p.ApplicantID, p.Summary, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrProfileExists
		}
		return insertLists(ctx, tx, p.ApplicantID, p.ProfileContent)
	})
}

// Get reads the profile and both lists in one read-only snapshot, so a
// concurrent replace is never seen half-done.
func (r *Profiles) Get(ctx context.Context, applicantID string) (domain.Profile, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Profile{}, hideDriverError(ctx, "get applicant profile failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p := domain.Profile{ApplicantID: applicantID}
	err = tx.QueryRow(ctx,
		`SELECT applicant_id, summary, created_at, updated_at FROM applicant_profiles WHERE applicant_id = $1`,
		applicantID).Scan(&p.ApplicantID, &p.Summary, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.Profile{}, hideDriverError(ctx, "get applicant profile failed")
	}
	if p.Education, p.Skills, err = readLists(ctx, tx, applicantID); err != nil {
		return domain.Profile{}, hideDriverError(ctx, "get applicant profile failed")
	}
	// Store and return UTC, as AGENTS.md Section 9 requires.
	p.CreatedAt, p.UpdatedAt = p.CreatedAt.UTC(), p.UpdatedAt.UTC()
	return p, nil
}

// Replace updates the summary and updated_at, then swaps both lists, in one
// transaction. The UPDATE locks the profile row, so concurrent replaces run
// one after the other and never mix their lists.
func (r *Profiles) Replace(ctx context.Context, applicantID string, c domain.ProfileContent, updatedAt time.Time) (domain.Profile, error) {
	p := domain.Profile{ApplicantID: applicantID, ProfileContent: c, UpdatedAt: updatedAt.UTC()}
	err := r.inTx(ctx, "update applicant profile failed", func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE applicant_profiles SET summary = $2, updated_at = $3
			 WHERE applicant_id = $1 RETURNING created_at`,
			applicantID, c.Summary, updatedAt).Scan(&p.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrProfileNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM profile_education WHERE applicant_id = $1`, applicantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM profile_skills WHERE applicant_id = $1`, applicantID); err != nil {
			return err
		}
		return insertLists(ctx, tx, applicantID, c)
	})
	if err != nil {
		return domain.Profile{}, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return p, nil
}

// insertLists stores each entry with its position so the order is kept.
func insertLists(ctx context.Context, tx pgx.Tx, applicantID string, c domain.ProfileContent) error {
	batch := &pgx.Batch{}
	for i, e := range c.Education {
		batch.Queue(`INSERT INTO profile_education (applicant_id, position, institution, qualification, start_year, end_year)
			VALUES ($1, $2, $3, $4, $5, $6)`, applicantID, i, e.Institution, e.Qualification, e.StartYear, e.EndYear)
	}
	for i, s := range c.Skills {
		batch.Queue(`INSERT INTO profile_skills (applicant_id, position, skill) VALUES ($1, $2, $3)`, applicantID, i, s)
	}
	if batch.Len() == 0 {
		return nil
	}
	return tx.SendBatch(ctx, batch).Close()
}

func readLists(ctx context.Context, tx pgx.Tx, applicantID string) ([]domain.Education, []string, error) {
	rows, err := tx.Query(ctx,
		`SELECT institution, qualification, start_year, end_year FROM profile_education
		 WHERE applicant_id = $1 ORDER BY position`, applicantID)
	if err != nil {
		return nil, nil, err
	}
	education, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Education, error) {
		var e domain.Education
		err := row.Scan(&e.Institution, &e.Qualification, &e.StartYear, &e.EndYear)
		return e, err
	})
	if err != nil {
		return nil, nil, err
	}
	rows, err = tx.Query(ctx,
		`SELECT skill FROM profile_skills WHERE applicant_id = $1 ORDER BY position`, applicantID)
	if err != nil {
		return nil, nil, err
	}
	skills, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	// Lists are never nil, as in domain.ProfileContent.
	if education == nil {
		education = []domain.Education{}
	}
	if skills == nil {
		skills = []string{}
	}
	return education, skills, nil
}

// inTx commits only if fn succeeds. Domain errors pass through; driver errors
// get a fixed message because they can contain SQL literals.
func (r *Profiles) inTx(ctx context.Context, message string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return hideDriverError(ctx, message)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		if errors.Is(err, domain.ErrProfileExists) || errors.Is(err, domain.ErrProfileNotFound) {
			return err
		}
		return hideDriverError(ctx, message)
	}
	if err := tx.Commit(ctx); err != nil {
		return hideDriverError(ctx, message)
	}
	return nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
