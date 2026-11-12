package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// write never contacts the broker: failed event storage rolls back the application.
func (r *Repository) write(ctx context.Context, withdrawn bool, change func(pgx.Tx) (domain.Application, error)) (domain.Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Application{}, err
	}
	defer tx.Rollback(context.Background())
	a, err := change(tx)
	if err != nil {
		return domain.Application{}, err
	}
	key := "application.submitted"
	var data proto.Message = &eventsv1.ApplicationSubmitted{ApplicationId: a.ID, ApplicantId: a.ApplicantID, JobId: a.JobID, ResumeId: a.ResumeID, EmployerId: a.EmployerID}
	if withdrawn {
		key = "application.withdrawn"
		data = &eventsv1.ApplicationWithdrawn{ApplicationId: a.ID, ApplicantId: a.ApplicantID, JobId: a.JobID}
	}
	body, err := anypb.New(data)
	if err != nil {
		return domain.Application{}, err
	}
	event := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: key, Producer: "application", OccurredAt: timestamppb.New(a.UpdatedAt), Data: body}
	payload, err := events.MarshalEnvelope(event)
	if err != nil {
		return domain.Application{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_id,routing_key,payload,created_at) VALUES($1,$2,$3,$4)`, event.EventId, key, string(payload), a.UpdatedAt); err != nil {
		return domain.Application{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Application{}, err
	}
	return a, nil
}
