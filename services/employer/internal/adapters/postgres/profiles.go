package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

const uniqueViolation = "23505"

// hasLogoColumn only checks that a logo row exists, so profile reads never
// load the image bytes (JT-102).
const hasLogoColumn = "EXISTS (SELECT 1 FROM company_logos l WHERE l.employer_id = company_profiles.employer_id)"

const profileColumns = "employer_id, company_name, description, website, location, verification_status, rejection_reason, created_at, updated_at, " + hasLogoColumn

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

// PutLogo updates the profile row first: it fails with ErrProfileNotFound
// when there is no profile, and its row lock makes concurrent uploads for one
// employer run one after the other. Only updated_at changes on the profile,
// never the verification columns (JT-102).
func (r *Profiles) PutLogo(ctx context.Context, employerID string, logo domain.Logo, now time.Time) (domain.CompanyProfile, error) {
	var updated domain.CompanyProfile
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE company_profiles SET updated_at = $2 WHERE employer_id = $1`, employerID, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrProfileNotFound
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO company_logos (employer_id, content_type, content, updated_at)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (employer_id) DO UPDATE
			 SET content_type = EXCLUDED.content_type, content = EXCLUDED.content, updated_at = EXCLUDED.updated_at`,
			employerID, logo.ContentType, logo.Content, now); err != nil {
			return err
		}
		updated, err = scanProfile(tx.QueryRow(ctx,
			`SELECT `+profileColumns+` FROM company_profiles WHERE employer_id = $1`, employerID))
		return err
	})
	if errors.Is(err, domain.ErrProfileNotFound) {
		return domain.CompanyProfile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.CompanyProfile{}, safe(ctx, "put company logo failed")
	}
	return updated, nil
}

// GetLogo is the only query that reads the image bytes.
func (r *Profiles) GetLogo(ctx context.Context, employerID string) (domain.Logo, error) {
	var logo domain.Logo
	err := r.pool.QueryRow(ctx,
		`SELECT content_type, content FROM company_logos WHERE employer_id = $1`, employerID,
	).Scan(&logo.ContentType, &logo.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Logo{}, domain.ErrLogoNotFound
	}
	if err != nil {
		return domain.Logo{}, safe(ctx, "get company logo failed")
	}
	return logo, nil
}

func scanProfile(row pgx.Row) (domain.CompanyProfile, error) {
	var p domain.CompanyProfile
	var status string
	if err := row.Scan(&p.EmployerID, &p.CompanyName, &p.Description, &p.Website, &p.Location,
		&status, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt, &p.HasLogo); err != nil {
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
