package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

// Verifications stores verification decisions. The adapter never owns the pool.
type Verifications struct{ pool *pgxpool.Pool }

var _ ports.VerificationRepository = (*Verifications)(nil)

func NewVerifications(pool *pgxpool.Pool) *Verifications { return &Verifications{pool: pool} }

// lockEmployer serialises ApplyVerification and Create for one employer, so a
// decision stored while the profile is being created is never lost.
const lockEmployer = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 56))`

func (r *Verifications) ApplyVerification(ctx context.Context, d domain.VerificationDecision, now time.Time) (domain.VerificationOutcome, error) {
	status, ok := statusText(d.Status)
	if !ok || d.Status == domain.VerificationUnverified {
		return 0, errors.New("decision status is not valid")
	}
	var outcome domain.VerificationOutcome
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockEmployer, d.EmployerID); err != nil {
			return err
		}
		claimed, err := tx.Exec(ctx,
			`INSERT INTO processed_events (event_id, processed_at) VALUES ($1, $2)
			 ON CONFLICT (event_id) DO NOTHING`, d.EventID, now)
		if err != nil {
			return err
		}
		if claimed.RowsAffected() == 0 {
			outcome = domain.OutcomeDuplicate
			return nil
		}
		// Equal times count as stale: the first decision stored wins.
		stored, err := tx.Exec(ctx,
			`INSERT INTO employer_verifications (employer_id, status, rejection_reason, decided_at, event_id)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (employer_id) DO UPDATE
			 SET status = EXCLUDED.status, rejection_reason = EXCLUDED.rejection_reason,
			     decided_at = EXCLUDED.decided_at, event_id = EXCLUDED.event_id
			 WHERE employer_verifications.decided_at < EXCLUDED.decided_at`,
			d.EmployerID, status, d.RejectionReason, d.DecidedAt, d.EventID)
		if err != nil {
			return err
		}
		if stored.RowsAffected() == 0 {
			// The event stays processed, so a redelivery is a duplicate.
			outcome = domain.OutcomeStale
			return nil
		}
		updated, err := tx.Exec(ctx,
			`UPDATE company_profiles
			 SET verification_status = $2, rejection_reason = $3, updated_at = $4
			 WHERE employer_id = $1`,
			d.EmployerID, status, d.RejectionReason, now)
		if err != nil {
			return err
		}
		outcome = domain.OutcomeApplied
		if updated.RowsAffected() == 0 {
			outcome = domain.OutcomeStoredWithoutProfile
		}
		return nil
	})
	if err != nil {
		return 0, safe(ctx, "apply verification decision failed")
	}
	return outcome, nil
}
