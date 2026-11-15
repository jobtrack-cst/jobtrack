package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

var _ ports.HiringEventRepository = (*Statuses)(nil)

// ApplyHiringEvent claims the event_id, locks the application's status row and
// applies domain.ApplyHiring. Every resulting step stores a history entry and an
// application.status_changed outbox row; the status row is updated once, only
// if it still holds the status that was read. A repeated event_id changes
// nothing, and a further interview claims the event_id without other writes.
// Any error rolls back, so the claim is released for a retry (JT-81).
func (r *Statuses) ApplyHiringEvent(ctx context.Context, e domain.HiringEvent) error {
	return r.inTx(ctx, "apply hiring event failed", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING", e.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		var s domain.ApplicationStatus
		var stored string
		err = tx.QueryRow(ctx, "SELECT "+statusColumns+" FROM application_statuses WHERE application_id=$1 FOR UPDATE", e.ApplicationID).
			Scan(&s.ApplicationID, &s.ApplicantID, &s.EmployerID, &stored, &s.CreatedAt, &s.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrApplicationNotFound
		}
		if err != nil {
			return err
		}
		if s.Status, err = parseStored(stored); err != nil {
			return err
		}
		updated, entries, err := s.ApplyHiring(e)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return nil
		}
		tag, err = tx.Exec(ctx, "UPDATE application_statuses SET status=$3,updated_at=$4 WHERE application_id=$1 AND status=$2",
			s.ApplicationID, s.Status.String(), updated.Status.String(), updated.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrStatusChanged
		}
		for _, entry := range entries {
			if err := insertEntry(ctx, tx, entry); err != nil {
				return err
			}
			if err := insertStatusEvent(ctx, tx, updated, entry); err != nil {
				return err
			}
		}
		return nil
	})
}
