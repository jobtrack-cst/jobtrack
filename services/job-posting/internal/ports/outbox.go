package ports

//go:generate go tool mockgen -destination=mocks/outbox_mock.go -package=mocks . OutboxRepository

import (
	"context"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// OutboxRepository reads the events that JobRepository stored, so
// the relay can publish them after the job change is committed (JT-68).
type OutboxRepository interface {
	// Pending returns up to limit unpublished events, oldest first.
	Pending(ctx context.Context, limit int) ([]*eventsv1.EventEnvelope, error)
	// MarkPublished records that the broker confirmed the event.
	MarkPublished(ctx context.Context, eventID string, at time.Time) error
}
