package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
)

// The same repository owns application writes and committed outbox reads.
var _ ports.OutboxRepository = (*Repository)(nil)

func (o *Repository) Pending(ctx context.Context, limit int) ([]*eventsv1.EventEnvelope, error) {
	if limit < 1 {
		return nil, errors.New("outbox limit must be positive")
	}
	rows, err := o.pool.Query(ctx,
		`SELECT payload FROM outbox_events
		 WHERE published_at IS NULL
		 ORDER BY created_at, event_id
		 LIMIT $1`, limit)
	if err != nil {
		return nil, safe(ctx, "list outbox events failed")
	}
	payloads, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, safe(ctx, "list outbox events failed")
	}
	pending := make([]*eventsv1.EventEnvelope, 0, len(payloads))
	for _, payload := range payloads {
		envelope, err := events.UnmarshalEnvelope([]byte(payload))
		if err != nil {
			return nil, errors.New("stored outbox event is not valid")
		}
		pending = append(pending, envelope)
	}
	return pending, nil
}

func (o *Repository) MarkPublished(ctx context.Context, eventID string, at time.Time) error {
	if _, err := o.pool.Exec(ctx,
		`UPDATE outbox_events SET published_at = $2
		 WHERE event_id = $1 AND published_at IS NULL`, eventID, at); err != nil {
		return safe(ctx, "mark outbox event published failed")
	}
	return nil
}

func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
