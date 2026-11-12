package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// Outbox borrows the Resume pool; it never closes it or publishes events.
type Outbox struct{ pool *pgxpool.Pool }

var _ ports.OutboxRepository = (*Outbox)(nil)

func NewOutbox(pool *pgxpool.Pool) *Outbox { return &Outbox{pool: pool} }

const outboxDatabaseTimeout = 2 * time.Second

func (r *Outbox) Claim(ctx context.Context, lease time.Duration) (domain.OutgoingEvent, bool, error) {
	if err := domain.ValidateOutboxLease(lease); err != nil {
		return domain.OutgoingEvent{}, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, outboxDatabaseTimeout)
	defer cancel()
	var event domain.OutgoingEvent
	found := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `WITH candidate AS (
 SELECT event_id FROM outgoing_events
 WHERE published_at IS NULL
   AND (lease_until IS NULL OR lease_until <= statement_timestamp())
   AND (next_attempt_at IS NULL OR next_attempt_at <= statement_timestamp())
 ORDER BY created_at, event_id LIMIT 1 FOR UPDATE SKIP LOCKED
 )
 UPDATE outgoing_events AS e
 SET claim_token = $1, lease_until = statement_timestamp() + $2::bigint * INTERVAL '1 microsecond'
 FROM candidate WHERE e.event_id = candidate.event_id
 RETURNING e.event_id, e.event_type, e.payload, e.claim_token, e.lease_until`,
			uuid.NewString(), lease.Microseconds()).Scan(&event.ID, &event.Type, &event.Payload, &event.ClaimToken, &event.LeaseUntil)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	if err != nil {
		return domain.OutgoingEvent{}, false, hideDriverError(ctx, "claim outgoing event failed")
	}
	event.LeaseUntil = event.LeaseUntil.UTC()
	return event, found, nil
}

func (r *Outbox) MarkPublished(ctx context.Context, eventID, token string) (bool, error) {
	if err := domain.ValidateOutboxClaim(eventID, token); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, outboxDatabaseTimeout)
	defer cancel()
	tag, err := r.pool.Exec(ctx, `UPDATE outgoing_events
 SET published_at = statement_timestamp(), claim_token = NULL, lease_until = NULL, next_attempt_at = NULL
 WHERE event_id = $1 AND claim_token = $2 AND published_at IS NULL`, eventID, token)
	if err != nil {
		return false, hideDriverError(ctx, "complete outgoing event failed")
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Outbox) Retry(ctx context.Context, eventID, token string, delay time.Duration) (bool, error) {
	if err := domain.ValidateOutboxClaim(eventID, token); err != nil {
		return false, err
	}
	if err := domain.ValidateOutboxRetryDelay(delay); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, outboxDatabaseTimeout)
	defer cancel()
	tag, err := r.pool.Exec(ctx, `UPDATE outgoing_events
 SET claim_token = NULL, lease_until = NULL,
 next_attempt_at = statement_timestamp() + $3::bigint * INTERVAL '1 microsecond'
 WHERE event_id = $1 AND claim_token = $2 AND published_at IS NULL`, eventID, token, delay.Microseconds())
	if err != nil {
		return false, hideDriverError(ctx, "retry outgoing event failed")
	}
	return tag.RowsAffected() == 1, nil
}
