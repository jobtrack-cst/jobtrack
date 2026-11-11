package rabbitmq

import (
	"context"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

type publisherBackend interface {
	ports.EventPublisher
	Close() error
}

// Publisher delegates transport and confirmations to the shared event publisher.
type Publisher struct{ backend publisherBackend }

var _ ports.EventPublisher = (*Publisher)(nil)

func NewPublisher(rabbitMQURL string) (*Publisher, error) {
	backend, err := events.NewPublisher(rabbitMQURL)
	if err != nil {
		return nil, err
	}
	return &Publisher{backend: backend}, nil
}

func (p *Publisher) Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
	return p.backend.Publish(ctx, envelope)
}

// Close releases the owned shared publisher, whose Close method is idempotent.
func (p *Publisher) Close() error { return p.backend.Close() }
