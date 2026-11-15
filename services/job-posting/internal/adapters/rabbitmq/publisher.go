package rabbitmq

import (
	"context"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

type eventPublisher interface {
	Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error
	Close() error
}

type Publisher struct {
	backend eventPublisher
}

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

func (p *Publisher) Close() error {
	return p.backend.Close()
}
