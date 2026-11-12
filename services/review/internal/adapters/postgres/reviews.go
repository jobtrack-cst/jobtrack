package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/ports"
)

// Participants reads the application_participants projection. The adapter
// never owns the pool.
type Participants struct{ pool *pgxpool.Pool }

var _ ports.ParticipantRepository = (*Participants)(nil)

func NewParticipants(pool *pgxpool.Pool) *Participants { return &Participants{pool: pool} }

func (r *Participants) GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipants, error) {
	var p domain.ApplicationParticipants
	err := r.pool.QueryRow(ctx,
		`SELECT application_id, applicant_id, employer_id FROM application_participants WHERE application_id = $1`,
		applicationID).Scan(&p.ApplicationID, &p.ApplicantID, &p.EmployerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationParticipants{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.ApplicationParticipants{}, hideDriverError(ctx, "get application participants failed")
	}
	return p, nil
}

const reviewColumns = "review_id, application_id, employer_id, score, decision, created_at, updated_at"

// Reviews stores completed reviews. The unique application_id makes "one
// review per application" hold even for simultaneous requests.
type Reviews struct{ pool *pgxpool.Pool }

var _ ports.ReviewRepository = (*Reviews)(nil)

func NewReviews(pool *pgxpool.Pool) *Reviews { return &Reviews{pool: pool} }

func (r *Reviews) Create(ctx context.Context, rv domain.Review) error {
	if _, ok := domain.ParseDecision(rv.Decision.String()); !ok {
		return errors.New("review decision is not valid")
	}
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO reviews (`+reviewColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (application_id) DO NOTHING`,
		rv.ReviewID, rv.ApplicationID, rv.EmployerID, rv.Score, rv.Decision.String(), rv.CreatedAt, rv.UpdatedAt)
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return hideDriverError(ctx, "create review failed")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrReviewExists
	}
	return nil
}

func (r *Reviews) GetByApplicationID(ctx context.Context, applicationID string) (domain.Review, error) {
	var rv domain.Review
	var decision string
	err := r.pool.QueryRow(ctx,
		`SELECT `+reviewColumns+` FROM reviews WHERE application_id = $1`, applicationID).
		Scan(&rv.ReviewID, &rv.ApplicationID, &rv.EmployerID, &rv.Score, &decision, &rv.CreatedAt, &rv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Review{}, domain.ErrReviewNotFound
	}
	if err != nil {
		return domain.Review{}, hideDriverError(ctx, "get review failed")
	}
	parsed, ok := domain.ParseDecision(decision)
	if !ok {
		return domain.Review{}, errors.New("stored review decision is not valid")
	}
	rv.Decision = parsed
	// Store and return UTC, as AGENTS.md Section 9 requires.
	rv.CreatedAt, rv.UpdatedAt = rv.CreatedAt.UTC(), rv.UpdatedAt.UTC()
	return rv, nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
