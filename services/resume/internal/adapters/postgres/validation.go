package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// Validation borrows the existing Resume pool and owns no transport resources.
type Validation struct{ pool *pgxpool.Pool }

var _ ports.ValidationRepository = (*Validation)(nil)

func NewValidation(pool *pgxpool.Pool) *Validation { return &Validation{pool: pool} }

func (r *Validation) ApplyValidation(ctx context.Context, input domain.ValidationResult, updatedAt time.Time) (domain.ValidationOutcome, error) {
	result, err := domain.NewValidationResult(input.EventID, input.ResumeID, input.ApplicantID, input.Status, input.Reason)
	if err != nil {
		return domain.ValidationOutcomeUnspecified, err
	}
	if updatedAt.IsZero() {
		return domain.ValidationOutcomeUnspecified, domain.ErrInvalidValidationResult
	}
	outcome := domain.ValidationOutcomeUnspecified
	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT(event_id) DO NOTHING`, result.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			outcome = domain.ValidationDuplicate
			return nil
		}
		var owner, status string
		err = tx.QueryRow(ctx, `SELECT applicant_id, validation_status FROM resumes WHERE resume_id=$1 FOR UPDATE`, result.ResumeID).Scan(&owner, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrResumeNotFound
		}
		if err != nil {
			return err
		}
		if owner != result.ApplicantID {
			return domain.ErrValidationApplicantMismatch
		}
		current, ok := domain.ParseValidationStatus(status)
		if !ok {
			return domain.ErrInvalidValidationResult
		}
		apply, err := current.AcceptsValidationResult(result.Status)
		if err != nil {
			return err
		}
		if !apply {
			outcome = domain.ValidationAlreadyFinal
			return nil
		}
		tag, err = tx.Exec(ctx, `UPDATE resumes SET validation_status=$1, rejection_reason=$2, updated_at=$3
 WHERE resume_id=$4 AND applicant_id=$5 AND validation_status='pending'`, result.Status.String(), result.Reason, updatedAt.UTC(), result.ResumeID, result.ApplicantID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("conditional validation update failed")
		}
		outcome = domain.ValidationApplied
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrResumeNotFound) || errors.Is(err, domain.ErrValidationApplicantMismatch) || errors.Is(err, domain.ErrInvalidValidationResult) {
			return domain.ValidationOutcomeUnspecified, err
		}
		return domain.ValidationOutcomeUnspecified, hideDriverError(ctx, "apply validation result failed")
	}
	return outcome, nil
}
