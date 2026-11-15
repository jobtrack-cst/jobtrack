package app

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
)

// OutboxBatchSize is how many events one PublishPending call reads.
const OutboxBatchSize = 50

// PublishTimeout bounds the wait for one broker confirmation.
const PublishTimeout = 10 * time.Second

// OutboxRelay publishes committed job changes (JT-68).
type OutboxRelay struct {
	repo      ports.OutboxRepository
	publisher ports.EventPublisher
	clock     ports.Clock
}

func NewOutboxRelay(repo ports.OutboxRepository, publisher ports.EventPublisher, clock ports.Clock) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, clock: clock}
}

// PublishPending publishes unpublished events oldest first and returns how
// many it published.
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
		if err := r.repo.MarkPublished(ctx, event.GetEventId(), r.clock.Now()); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}
