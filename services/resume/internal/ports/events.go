package ports

import (
	"context"
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

//go:generate go tool mockgen -destination=mocks/events_mock.go -package=mocks . EventPublisher

// ErrPublisherUnusable requires process restart after publisher cleanup fails.
var ErrPublisherUnusable = errors.New("RabbitMQ publisher cleanup failed; restart required")

// EventPublisher publishes existing shared domain event envelopes.
type EventPublisher interface {
	Publish(context.Context, *eventsv1.EventEnvelope) error
}
