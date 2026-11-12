package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
)

// OutboxBatchSize is how many events one PublishPending call reads.
const OutboxBatchSize = 50

// OutboxRelay publishes the events stored by Decide, after their decisions
// are committed (JT-56).
type OutboxRelay struct {
	repo      ports.OutboxRepository
	publisher ports.EventPublisher
	clock     ports.Clock
}

func NewOutboxRelay(repo ports.OutboxRepository, publisher ports.EventPublisher, clock ports.Clock) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, clock: clock}
}

// PublishPending publishes unpublished events oldest first and returns how
// many it published. It stops at the first failure, so a later decision is
// never published before an earlier one; the next call retries from there.
// If marking fails after a confirmed publish, the event is published again
// later, which the idempotent employer consumer ignores.
func (r *OutboxRelay) PublishPending(ctx context.Context) (int, error) {
	pending, err := r.repo.Pending(ctx, OutboxBatchSize)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, event := range pending {
		if err := r.publisher.Publish(ctx, event); err != nil {
			return published, err
		}
		if err := r.repo.MarkPublished(ctx, event.GetEventId(), r.clock.Now()); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}
