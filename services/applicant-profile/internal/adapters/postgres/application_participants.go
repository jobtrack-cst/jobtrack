package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports"
)

// ApplicationParticipants stores the application.submitted projection
// (JT-114 Part C). It borrows the service's pool and never owns its lifecycle.
type ApplicationParticipants struct{ pool *pgxpool.Pool }

var _ ports.ApplicationParticipantRepository = (*ApplicationParticipants)(nil)

func NewApplicationParticipants(pool *pgxpool.Pool) *ApplicationParticipants {
	return &ApplicationParticipants{pool: pool}
}

var errConflictingApplication = errors.New("conflicting application participants")

// Record claims the event and stores the immutable row in one transaction.
func (r *ApplicationParticipants) Record(ctx context.Context, eventID string, row domain.ApplicationParticipant) error {
	eventID, err := domain.NormalizeEventID(eventID)
	if err != nil {
		return events.Permanent(err)
	}
	row, err = domain.NewApplicationParticipant(row.ApplicationID, row.ApplicantID, row.EmployerID)
	if err != nil {
		return events.Permanent(err)
	}
	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT(event_id) DO NOTHING`, eventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil // repeated event_id: already applied
		}
		// The no-op update reports one row only when the stored participants
		// match, so a redelivery under a new event_id is accepted.
		tag, err = tx.Exec(ctx, `INSERT INTO application_participants(application_id, applicant_id, employer_id)
 VALUES($1, $2, $3)
 ON CONFLICT(application_id) DO UPDATE SET applicant_id = EXCLUDED.applicant_id
 WHERE application_participants.applicant_id = EXCLUDED.applicant_id
   AND application_participants.employer_id = EXCLUDED.employer_id`,
			row.ApplicationID, row.ApplicantID, row.EmployerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errConflictingApplication // rolls back the claim
		}
		return nil
	})
	if errors.Is(err, errConflictingApplication) {
		return events.Permanent(err)
	}
	if err != nil {
		// Retried under the JT-61 policy; driver text may hold connection details.
		return hideDriverError(ctx, "record application participants failed")
	}
	return nil
}

func (r *ApplicationParticipants) GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipant, error) {
	var row domain.ApplicationParticipant
	err := r.pool.QueryRow(ctx,
		`SELECT application_id, applicant_id, employer_id FROM application_participants WHERE application_id = $1`,
		applicationID).Scan(&row.ApplicationID, &row.ApplicantID, &row.EmployerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationParticipant{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.ApplicationParticipant{}, hideDriverError(ctx, "get application participants failed")
	}
	return row, nil
}
