package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
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

// RecordApplication handles one application.submitted event: it claims the
// event_id in processed_events and stores the participants in the same
// transaction (JT-79, the JT-74 Offer design). A repeated event_id changes
// nothing. Participants never change: different participants for a stored
// application return domain.ErrConflictingParticipants and keep no claim.
func (r *Participants) RecordApplication(ctx context.Context, eventID string, p domain.ApplicationParticipants) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return hideDriverError(ctx, "record application failed")
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	claimed, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, eventID)
	if err != nil {
		return hideDriverError(ctx, "record application failed")
	}
	if claimed.RowsAffected() == 0 {
		return nil
	}
	// The no-op update returns a row only when the stored participants match,
	// so a second event for the same application is accepted unchanged.
	stored, err := tx.Exec(ctx,
		`INSERT INTO application_participants (application_id, applicant_id, employer_id) VALUES ($1, $2, $3)
		 ON CONFLICT (application_id) DO UPDATE SET applicant_id = EXCLUDED.applicant_id
		 WHERE application_participants.applicant_id = EXCLUDED.applicant_id
		   AND application_participants.employer_id = EXCLUDED.employer_id`,
		p.ApplicationID, p.ApplicantID, p.EmployerID)
	if err != nil {
		return hideDriverError(ctx, "record application failed")
	}
	if stored.RowsAffected() == 0 {
		return domain.ErrConflictingParticipants
	}
	if err := tx.Commit(ctx); err != nil {
		return hideDriverError(ctx, "record application failed")
	}
	return nil
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return hideDriverError(ctx, "create review failed")
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	tag, err := tx.Exec(ctx,
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
	if err := storeReviewCompleted(ctx, tx, rv); err != nil {
		return hideDriverError(ctx, "create review failed")
	}
	if err := tx.Commit(ctx); err != nil {
		return hideDriverError(ctx, "create review failed")
	}
	return nil
}

// storeReviewCompleted adds review.completed to the outbox inside the review's
// transaction, so the event exists exactly when the review is committed.
func storeReviewCompleted(ctx context.Context, tx pgx.Tx, rv domain.Review) error {
	event, err := app.ReviewCompletedEvent(rv, uuid.NewString())
	if err != nil {
		return err
	}
	payload, err := events.MarshalEnvelope(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox_events (event_id, routing_key, payload, created_at) VALUES ($1, $2, $3, $4)`,
		event.GetEventId(), event.GetType(), string(payload), rv.CreatedAt)
	return err
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
