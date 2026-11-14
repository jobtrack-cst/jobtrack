package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports"
)

type Offers struct{ pool *pgxpool.Pool }

var _ ports.OfferRepository = (*Offers)(nil)

func NewOffers(pool *pgxpool.Pool) *Offers { return &Offers{pool: pool} }

const offerColumns = "offer_id, application_id, employer_id, applicant_id, terms, status, created_at, updated_at"

func (r *Offers) Create(ctx context.Context, o domain.Offer) (domain.Offer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	defer tx.Rollback(ctx)
	created, err := scanOffer(tx.QueryRow(ctx, `INSERT INTO offers (`+offerColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+offerColumns, o.ID, o.ApplicationID, o.EmployerID, o.ApplicantID, o.Terms, statusText(o.Status), o.CreatedAt, o.UpdatedAt))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "offers_application_id_key" {
		return domain.Offer{}, domain.ErrOfferExists
	}
	if err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	if err := storeOfferEvent(ctx, tx, created); err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	return created, nil
}

func (r *Offers) GetByID(ctx context.Context, rawID string) (domain.Offer, error) {
	id, err := domain.NormalizeID(rawID, domain.ErrInvalidOfferID)
	if err != nil {
		return domain.Offer{}, err
	}
	o, err := scanOffer(r.pool.QueryRow(ctx, `SELECT `+offerColumns+` FROM offers WHERE offer_id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Offer{}, domain.ErrOfferNotFound
	}
	if err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	return o, nil
}

func (r *Offers) Decide(ctx context.Context, id, applicantID string, next domain.OfferStatus, at time.Time) (domain.Offer, error) {
	if next != domain.OfferStatusAccepted && next != domain.OfferStatusDeclined {
		return domain.Offer{}, domain.ErrOfferNotPending
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	defer tx.Rollback(ctx)
	// PostgreSQL rechecks the predicate after waiting for a concurrent writer.
	o, err := scanOffer(tx.QueryRow(ctx, `UPDATE offers SET status=$3,updated_at=$4 WHERE offer_id=$1 AND applicant_id=$2 AND status='pending' RETURNING `+offerColumns, id, applicantID, statusText(next), at.UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		// Reuse the transaction's connection, including when the pool has only one.
		existing, err := scanOffer(tx.QueryRow(ctx, `SELECT `+offerColumns+` FROM offers WHERE offer_id=$1`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Offer{}, domain.ErrOfferNotFound
		}
		if err != nil {
			return domain.Offer{}, offerDBError(ctx)
		}
		if existing.ApplicantID != applicantID {
			return domain.Offer{}, domain.ErrPermissionDenied
		}
		return domain.Offer{}, domain.ErrOfferNotPending
	}
	if err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	if err := storeOfferEvent(ctx, tx, o); err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Offer{}, offerDBError(ctx)
	}
	return o, nil
}
func statusText(s domain.OfferStatus) string {
	switch s {
	case domain.OfferStatusPending:
		return "pending"
	case domain.OfferStatusAccepted:
		return "accepted"
	case domain.OfferStatusDeclined:
		return "declined"
	}
	return ""
}
func scanOffer(row pgx.Row) (domain.Offer, error) {
	var o domain.Offer
	var status string
	if err := row.Scan(&o.ID, &o.ApplicationID, &o.EmployerID, &o.ApplicantID, &o.Terms, &status, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return domain.Offer{}, err
	}
	switch status {
	case "pending":
		o.Status = domain.OfferStatusPending
	case "accepted":
		o.Status = domain.OfferStatusAccepted
	case "declined":
		o.Status = domain.OfferStatusDeclined
	default:
		return domain.Offer{}, errors.New("invalid stored offer status")
	}
	o.CreatedAt, o.UpdatedAt = o.CreatedAt.UTC(), o.UpdatedAt.UTC()
	return o, nil
}
func offerDBError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("offer database operation failed")
}

func storeOfferEvent(ctx context.Context, tx pgx.Tx, o domain.Offer) error {
	e, err := app.OfferEvent(o, uuid.NewString())
	if err != nil {
		return err
	}
	payload, err := events.MarshalEnvelope(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_id,routing_key,payload,created_at) VALUES($1,$2,$3,$4)`, e.EventId, e.Type, string(payload), o.UpdatedAt)
	return err
}
