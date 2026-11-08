package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/outcomes_mock.go -package=mocks . OutcomeRepository

// OutcomeRepository stores counted events and the processed event IDs that
// make the Sprint 2 consumer idempotent.
type OutcomeRepository interface {
	// RecordEvent marks the event_id processed and stores the outcome event in
	// one transaction. recorded is false, with no error, when the event_id was
	// already processed, so the consumer can acknowledge the duplicate.
	RecordEvent(ctx context.Context, event domain.OutcomeEvent) (recorded bool, err error)
	// CountEvents returns one row per event type, and per new_status for
	// status changes, for events with start <= occurred_at < end. Types with
	// no events in the range have no row.
	CountEvents(ctx context.Context, r domain.TimeRange) ([]domain.EventCount, error)
}
