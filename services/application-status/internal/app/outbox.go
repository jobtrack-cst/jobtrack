package app

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

// OutboxBatchSize is how many events one PublishPending call reads.
const OutboxBatchSize = 50

// PublishTimeout bounds the wait for one broker confirmation, so a broker
// that stops answering cannot stall the relay.
const PublishTimeout = 10 * time.Second

// OutboxRelay publishes committed status changes (JT-60).
type OutboxRelay struct {
	repo      ports.OutboxRepository
	publisher ports.EventPublisher
	clock     func() time.Time
}

func NewOutboxRelay(repo ports.OutboxRepository, publisher ports.EventPublisher, clock func() time.Time) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, clock: clock}
}

// PublishPending publishes unpublished events oldest first and returns how
// many it published. It stops at the first failure, so a later event is
// never published before an earlier event; the next call retries from there.
// If marking fails after a confirmed publish, the event is published again
// later, which the idempotent downstream consumer ignores.
func (r *OutboxRelay) PublishPending(ctx context.Context) (int, error) {
	pending, err := r.repo.Pending(ctx, OutboxBatchSize)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, event := range pending {
		publishCtx, cancel := context.WithTimeout(ctx, PublishTimeout)
		err := r.publisher.Publish(publishCtx, event)
		cancel()
		if err != nil {
			return published, err
		}
		if err := r.repo.MarkPublished(ctx, event.GetEventId(), r.clock()); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}
