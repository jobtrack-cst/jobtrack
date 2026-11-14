package postgres

import (
	"context"
	"errors"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports"
)

// Participants borrows the service's existing pool; it never owns its lifecycle.
type Participants struct{ pool *pgxpool.Pool }

var _ ports.ParticipantRepository = (*Participants)(nil)

func NewParticipants(pool *pgxpool.Pool) *Participants { return &Participants{pool: pool} }

var applicationIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (r *Participants) GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipants, error) {
	if !applicationIDPattern.MatchString(applicationID) {
		return domain.ApplicationParticipants{}, domain.ErrInvalidApplicationID
	}
	if err := ctx.Err(); err != nil {
		return domain.ApplicationParticipants{}, err
	}
	if r.pool == nil {
		return domain.ApplicationParticipants{}, errors.New("participant repository unavailable")
	}
	var participants domain.ApplicationParticipants
	err := r.pool.QueryRow(ctx,
		`SELECT application_id, applicant_id, employer_id FROM application_participants WHERE application_id = $1`,
		strings.ToLower(applicationID)).Scan(&participants.ApplicationID, &participants.ApplicantID, &participants.EmployerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ApplicationParticipants{}, domain.ErrParticipantsNotFound
	}
	if err != nil {
		if err := ctx.Err(); err != nil {
			return domain.ApplicationParticipants{}, err
		}
		// Driver errors may contain sensitive connection or query details.
		return domain.ApplicationParticipants{}, errors.New("get application participants failed")
	}
	return participants, nil
}

// RecordApplication claims the event and stores immutable participants together.
// Conflicting participants fail without retaining the deduplication claim.
func (r *Participants) RecordApplication(ctx context.Context, eventID string, p domain.ApplicationParticipants) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return offerDBError(ctx)
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING`, eventID)
	if err != nil {
		return offerDBError(ctx)
	}
	if result.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	result, err = tx.Exec(ctx, `INSERT INTO application_participants(application_id,applicant_id,employer_id) VALUES($1,$2,$3) ON CONFLICT(application_id) DO UPDATE SET applicant_id=EXCLUDED.applicant_id WHERE application_participants.applicant_id=EXCLUDED.applicant_id AND application_participants.employer_id=EXCLUDED.employer_id`, p.ApplicationID, p.ApplicantID, p.EmployerID)
	if err != nil {
		return offerDBError(ctx)
	}
	if result.RowsAffected() == 0 {
		return events.Permanent(errors.New("conflicting application participants"))
	}
	if err := tx.Commit(ctx); err != nil {
		return offerDBError(ctx)
	}
	return nil
}
