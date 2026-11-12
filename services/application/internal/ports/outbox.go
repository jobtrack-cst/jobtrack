package ports

//go:generate go tool mockgen -destination=mocks/outbox_mock.go -package=mocks . OutboxRepository,EventPublisher,Clock

import (
	"context"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// OutboxRepository reads the events that Repository.Create and Withdraw stored, so
// the relay can publish them after the application change is committed (JT-58).
type OutboxRepository interface {
	// Pending returns up to limit unpublished events, oldest first.
	Pending(ctx context.Context, limit int) ([]*eventsv1.EventEnvelope, error)
	// MarkPublished records that the broker confirmed the event. Marking an
	// event that is already marked is not an error.
	MarkPublished(ctx context.Context, eventID string, at time.Time) error
}

// Clock supplies the relay acknowledgement time.
type Clock interface{ Now() time.Time }
