package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/ports"
)

// Outcomes stores counted events and processed event IDs in the analytics
// database. The adapter never owns the pool.
type Outcomes struct{ pool *pgxpool.Pool }

var _ ports.OutcomeRepository = (*Outcomes)(nil)

func NewOutcomes(pool *pgxpool.Pool) *Outcomes { return &Outcomes{pool: pool} }

// RecordEvent claims the event_id first. When two deliveries of the same
// event race, the second insert waits for the first transaction and then
// does nothing, so exactly one row is stored. If the event insert fails, the
// claim rolls back too and a redelivery can try again.
func (r *Outcomes) RecordEvent(ctx context.Context, e domain.OutcomeEvent) (bool, error) {
	recorded := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO processed_events (event_id) VALUES ($1)
			 ON CONFLICT (event_id) DO NOTHING`, e.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outcome_events (event_id, event_type, new_status, occurred_at) VALUES ($1, $2, $3, $4)`,
			e.EventID, e.EventType, nullableStatus(e.NewStatus), e.OccurredAt); err != nil {
			return err
		}
		recorded = true
		return nil
	})
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return false, safe(ctx, "record outcome event failed")
	}
	return recorded, nil
}

// CountEvents uses the rounded bounds from QueryBounds, so start stays
// inclusive and end exclusive at microsecond precision. One statement, so
// the counts come from one snapshot.
func (r *Outcomes) CountEvents(ctx context.Context, tr domain.TimeRange) ([]domain.EventCount, error) {
	start, end := tr.QueryBounds()
	rows, err := r.pool.Query(ctx,
		`SELECT event_type, COALESCE(new_status, ''), count(*)
		 FROM outcome_events
		 WHERE occurred_at >= $1 AND occurred_at < $2
		 GROUP BY event_type, new_status
		 ORDER BY event_type, new_status`, start, end)
	if err != nil {
		return nil, safe(ctx, "count outcome events failed")
	}
	counts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.EventCount, error) {
		var c domain.EventCount
		err := row.Scan(&c.EventType, &c.NewStatus, &c.Count)
		return c, err
	})
	if err != nil {
		return nil, safe(ctx, "count outcome events failed")
	}
	return counts, nil
}

// nullableStatus stores NULL for events that are not status changes, as the
// table CHECK requires.
func nullableStatus(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
