package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// write never contacts the broker: failed event storage rolls back the job change.
func (r *Jobs) write(ctx context.Context, routingKey string, change func(pgx.Tx) (domain.Job, error)) (domain.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Job{}, err
	}
	defer tx.Rollback(context.Background())
	j, err := change(tx)
	if err != nil {
		return domain.Job{}, err
	}

	var data proto.Message
	switch routingKey {
	case "job.published":
		data = &eventsv1.JobPublished{
			JobId:      j.ID,
			EmployerId: j.EmployerID,
			Title:      j.Title,
			CategoryId: j.CategoryID,
			TagIds:     j.TagIDs,
			Location:   j.Location,
			Deadline:   timestamppb.New(j.Deadline),
		}
	case "job.updated":
		data = &eventsv1.JobUpdated{
			JobId:      j.ID,
			EmployerId: j.EmployerID,
			Title:      j.Title,
			CategoryId: j.CategoryID,
			TagIds:     j.TagIDs,
			Location:   j.Location,
			Deadline:   timestamppb.New(j.Deadline),
		}
	case "job.closed":
		data = &eventsv1.JobClosed{
			JobId: j.ID,
		}
	default:
		return domain.Job{}, errors.New("unknown routing key")
	}

	body, err := anypb.New(data)
	if err != nil {
		return domain.Job{}, err
	}
	event := &eventsv1.EventEnvelope{
		EventId:    uuid.NewString(),
		Type:       routingKey,
		Producer:   "job-posting",
		OccurredAt: timestamppb.New(j.UpdatedAt),
		Data:       body,
	}
	payload, err := events.MarshalEnvelope(event)
	if err != nil {
		return domain.Job{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_id,routing_key,payload,created_at) VALUES($1,$2,$3,$4)`, event.EventId, routingKey, string(payload), j.UpdatedAt); err != nil {
		return domain.Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Job{}, err
	}
	return j, nil
}
