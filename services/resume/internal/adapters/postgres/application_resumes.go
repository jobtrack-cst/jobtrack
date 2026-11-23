package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// ApplicationResumes stores the application.submitted projection (JT-114
// Part B). It borrows the service's pool and never owns its lifecycle.
type ApplicationResumes struct{ pool *pgxpool.Pool }

var _ ports.ApplicationResumeRepository = (*ApplicationResumes)(nil)

func NewApplicationResumes(pool *pgxpool.Pool) *ApplicationResumes {
	return &ApplicationResumes{pool: pool}
}

var errConflictingApplication = errors.New("conflicting application resume")

// Record claims the event and stores the immutable row in one transaction.
// The JT-59 consumer shares processed_events; event IDs are unique UUIDs, so
// the two consumers never claim each other's events.
func (r *ApplicationResumes) Record(ctx context.Context, eventID string, row domain.ApplicationResume) error {
	eventID, err := domain.NormalizeEventID(eventID)
	if err != nil {
		return events.Permanent(err)
	}
	row, err = domain.NewApplicationResume(row.ApplicationID, row.ApplicantID, row.EmployerID, row.ResumeID)
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
		tag, err = tx.Exec(ctx, `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id)
 VALUES($1, $2, $3, $4)
 ON CONFLICT(application_id) DO UPDATE SET applicant_id = EXCLUDED.applicant_id
 WHERE application_resumes.applicant_id = EXCLUDED.applicant_id
   AND application_resumes.employer_id = EXCLUDED.employer_id
   AND application_resumes.resume_id = EXCLUDED.resume_id`,
			row.ApplicationID, row.ApplicantID, row.EmployerID, row.ResumeID)
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
		return hideDriverError(ctx, "record application resume failed")
	}
	return nil
}

func (r *ApplicationResumes) GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationResume, error) {
	var row domain.ApplicationResume
	err := r.pool.QueryRow(ctx,
		`SELECT application_id, applicant_id, employer_id, resume_id FROM application_resumes WHERE application_id = $1`,
		applicationID).Scan(&row.ApplicationID, &row.ApplicantID, &row.EmployerID, &row.ResumeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationResume{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.ApplicationResume{}, hideDriverError(ctx, "get application resume failed")
	}
	return row, nil
}
