package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

// RecordEventInput holds the fields of a consumed event that analytics counts.
type RecordEventInput struct {
	EventID    string
	EventType  string
	NewStatus  string
	OccurredAt time.Time
}

// RecordEvent validates and stores one consumed event. recorded is false,
// with no error, when the event_id was already processed. A validation error
// wraps domain.ErrInvalidOutcomeEvent, so the consumer dead-letters the event
// instead of retrying it.
func (a *Analytics) RecordEvent(ctx context.Context, in RecordEventInput) (bool, error) {
	event, err := domain.NewOutcomeEvent(in.EventID, in.EventType, in.NewStatus, in.OccurredAt)
	if err != nil {
		return false, fmt.Errorf("%w: %w", domain.ErrInvalidOutcomeEvent, err)
	}
	recorded, err := a.repo.RecordEvent(ctx, event)
	if err != nil {
		return false, fmt.Errorf("record outcome event: %w", err)
	}
	return recorded, nil
}
