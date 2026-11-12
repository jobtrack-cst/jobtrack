package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
)

// Outbox reads and marks the events that Requests.Decide stored. The adapter
// never owns the pool.
type Outbox struct{ pool *pgxpool.Pool }

var _ ports.OutboxRepository = (*Outbox)(nil)

func NewOutbox(pool *pgxpool.Pool) *Outbox { return &Outbox{pool: pool} }

func (o *Outbox) Pending(ctx context.Context, limit int) ([]*eventsv1.EventEnvelope, error) {
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

func (o *Outbox) MarkPublished(ctx context.Context, eventID string, at time.Time) error {
	if _, err := o.pool.Exec(ctx,
		`UPDATE outbox_events SET published_at = $2
		 WHERE event_id = $1 AND published_at IS NULL`, eventID, at); err != nil {
		return safe(ctx, "mark outbox event published failed")
	}
	return nil
}
