package ports

import (
	"context"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// EventPublisher publishes existing shared domain event envelopes.
type EventPublisher interface {
	Publish(context.Context, *eventsv1.EventEnvelope) error
}
