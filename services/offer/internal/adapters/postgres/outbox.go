package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports"
)

// Outbox reads and marks the events that Offers mutations stored. The adapter
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
		 ORDER BY sequence
		 LIMIT $1`, limit)
	if err != nil {
		return nil, offerDBError(ctx)
	}
	payloads, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, offerDBError(ctx)
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
		return offerDBError(ctx)
	}
	return nil
}
