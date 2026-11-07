package postgres

import (
	"context"
	"errors"
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
