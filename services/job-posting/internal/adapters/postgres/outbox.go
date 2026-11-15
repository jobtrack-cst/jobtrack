package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) Pending(ctx context.Context, limit int) ([]*eventsv1.EventEnvelope, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, routing_key, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at, event_id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*eventsv1.EventEnvelope
	for rows.Next() {
		var eventID, routingKey, payload string
		if err := rows.Scan(&eventID, &routingKey, &payload); err != nil {
			return nil, err
		}
		env, err := events.UnmarshalEnvelope([]byte(payload))
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	return out, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = $1
		WHERE event_id = $2 AND published_at IS NULL`, at, eventID)
	return err
}

// SaveEvent is used inside an existing database transaction to record the event.
func SaveEvent(ctx context.Context, tx pgx.Tx, eventID string, routingKey string, payload []byte, occurredAt time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (event_id, routing_key, payload, created_at)
		VALUES ($1, $2, $3, $4)`,
		eventID, routingKey, string(payload), occurredAt)
	return err
}
