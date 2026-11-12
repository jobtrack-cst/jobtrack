package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ApplyEvent claims the input ID before touching state; rollback releases the claim.
func (r *Statuses) ApplyEvent(ctx context.Context, e domain.ApplicationEvent) error {
	return r.inTx(ctx, "apply application event failed", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING", e.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		var s domain.ApplicationStatus
		var entry domain.HistoryEntry
		if e.Submitted {
			s, entry, err = domain.NewSubmitted(e.ApplicationID, e.ApplicantID, e.EmployerID, e.OccurredAt)
			if err != nil {
				return domain.ErrConflictingEvent
			}
			tag, err = tx.Exec(ctx, "INSERT INTO application_statuses ("+statusColumns+") VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", s.ApplicationID, s.ApplicantID, s.EmployerID, s.Status.String(), s.CreatedAt, s.UpdatedAt)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrConflictingEvent
			}
		} else {
			var st string
			err = tx.QueryRow(ctx, "SELECT "+statusColumns+" FROM application_statuses WHERE application_id=$1 FOR UPDATE", e.ApplicationID).Scan(&s.ApplicationID, &s.ApplicantID, &s.EmployerID, &st, &s.CreatedAt, &s.UpdatedAt)
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrApplicationNotFound
			}
			if err != nil {
				return err
			}
			applicant, err := uuid.Parse(e.ApplicantID)
			if err != nil || applicant.String() != s.ApplicantID {
				return domain.ErrConflictingEvent
			}
			s.Status, err = parseStored(st)
			if err != nil {
				return err
			}
			expected := s.Status
			s, entry, err = s.Transition(domain.StatusWithdrawn, e.OccurredAt)
			if err != nil {
				return err
			}
			tag, err = tx.Exec(ctx, "UPDATE application_statuses SET status=$3,updated_at=$4 WHERE application_id=$1 AND status=$2", s.ApplicationID, expected.String(), s.Status.String(), s.UpdatedAt)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return domain.ErrStatusChanged
			}
		}
		if err = insertEntry(ctx, tx, entry); err != nil {
			return err
		}
		return insertStatusEvent(ctx, tx, s, entry)
	})
}

// The serialized envelope is retained so uncertain acknowledgements reuse its ID.
func insertStatusEvent(ctx context.Context, tx pgx.Tx, s domain.ApplicationStatus, e domain.HistoryEntry) error {
	data, err := anypb.New(&eventsv1.StatusChanged{ApplicationId: s.ApplicationID, ApplicantId: s.ApplicantID, PreviousStatus: eventStatus(e.PreviousStatus), NewStatus: eventStatus(e.NewStatus)})
	if err != nil {
		return err
	}
	envelope := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "application.status_changed", Producer: "application-status", OccurredAt: timestamppb.New(e.OccurredAt.UTC()), Data: data}
	payload, err := events.MarshalEnvelope(envelope)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO outbox_events(event_id,payload,created_at) VALUES($1,$2,$3)", envelope.EventId, string(payload), e.OccurredAt)
	return err
}

// Event strings follow the existing contract test's unprefixed uppercase names.
func eventStatus(s domain.Status) string {
	if s == domain.StatusUnspecified {
		return "UNSPECIFIED"
	}
	return strings.ToUpper(s.String())
}
