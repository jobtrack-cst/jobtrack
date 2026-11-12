package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

const uniqueViolation = "23505"

const profileColumns = "employer_id, company_name, description, website, location, verification_status, rejection_reason, created_at, updated_at"

// Profiles stores company profiles in the employer database. The adapter
// never owns the pool.
type Profiles struct{ pool *pgxpool.Pool }

var _ ports.ProfileRepository = (*Profiles)(nil)

func NewProfiles(pool *pgxpool.Pool) *Profiles { return &Profiles{pool: pool} }

// Create copies a verification decision that arrived before the profile
// (JT-56). The employer lock makes ApplyVerification wait, so a decision
// committed during Create is either copied here or applied to the new row.
func (r *Profiles) Create(ctx context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) {
	status, ok := statusText(p.VerificationStatus)
	if !ok {
		return domain.CompanyProfile{}, errors.New("verification status is not valid")
	}
	var created domain.CompanyProfile
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockEmployer, p.EmployerID); err != nil {
			return err
		}
		var err error
		created, err = scanProfile(tx.QueryRow(ctx,
			`INSERT INTO company_profiles (employer_id, company_name, description, website, location,
			     verification_status, rejection_reason, created_at, updated_at)
			 SELECT $1, $2, $3, $4, $5,
			     COALESCE(v.status, $6), COALESCE(v.rejection_reason, $7), $8, $9
			 FROM (SELECT 1) AS one
			 LEFT JOIN employer_verifications v ON v.employer_id = $1
			 RETURNING `+profileColumns,
			p.EmployerID, p.CompanyName, p.Description, p.Website, p.Location,
			status, p.RejectionReason, p.CreatedAt, p.UpdatedAt))
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.CompanyProfile{}, domain.ErrProfileExists
	}
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.CompanyProfile{}, safe(ctx, "create company profile failed")
	}
	return created, nil
}

func (r *Profiles) GetByEmployerID(ctx context.Context, employerID string) (domain.CompanyProfile, error) {
	profile, err := scanProfile(r.pool.QueryRow(ctx,
		`SELECT `+profileColumns+` FROM company_profiles WHERE employer_id = $1`, employerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CompanyProfile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.CompanyProfile{}, safe(ctx, "get company profile failed")
	}
	return profile, nil
}

// Update leaves verification_status, rejection_reason and created_at alone,
// so a verification event written between read and update is never lost.
func (r *Profiles) Update(ctx context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) {
	updated, err := scanProfile(r.pool.QueryRow(ctx,
		`UPDATE company_profiles
		 SET company_name = $2, description = $3, website = $4, location = $5, updated_at = $6
		 WHERE employer_id = $1
		 RETURNING `+profileColumns,
		p.EmployerID, p.CompanyName, p.Description, p.Website, p.Location, p.UpdatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CompanyProfile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.CompanyProfile{}, safe(ctx, "update company profile failed")
	}
	return updated, nil
}

func scanProfile(row pgx.Row) (domain.CompanyProfile, error) {
	var p domain.CompanyProfile
	var status string
	if err := row.Scan(&p.EmployerID, &p.CompanyName, &p.Description, &p.Website, &p.Location,
		&status, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.CompanyProfile{}, err
	}
	parsed, ok := parseStatus(status)
	if !ok {
		return domain.CompanyProfile{}, errors.New("stored verification status is not valid")
	}
	p.VerificationStatus = parsed
	// Store and return UTC, as AGENTS.md Section 9 requires.
	p.CreatedAt, p.UpdatedAt = p.CreatedAt.UTC(), p.UpdatedAt.UTC()
	return p, nil
}

var statusNames = map[domain.VerificationStatus]string{
	domain.VerificationUnverified: "unverified",
	domain.VerificationVerified:   "verified",
	domain.VerificationRejected:   "rejected",
}

func statusText(s domain.VerificationStatus) (string, bool) {
	name, ok := statusNames[s]
	return name, ok
}

func parseStatus(name string) (domain.VerificationStatus, bool) {
	for s, n := range statusNames {
		if n == name {
			return s, true
		}
	}
	return domain.VerificationUnspecified, false
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
