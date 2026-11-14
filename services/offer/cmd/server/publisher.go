package main

import (
	"context"
	"errors"
	"sync"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// recoveringPublisher drops the RabbitMQ connection after a failed publish
// and opens a new one on the next publish. The shared pkg/events publisher
// never reconnects, so without this a broker restart would leave events in
// the outbox until offer restarted (same approach as JT-59).
type recoveringPublisher struct {
	mu      sync.Mutex
	open    func() (publisherResource, error)
	current publisherResource
	closed  bool
}

func newRecoveringPublisher(first publisherResource, open func() (publisherResource, error)) *recoveringPublisher {
	return &recoveringPublisher{current: first, open: open}
}

func (p *recoveringPublisher) Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("RabbitMQ publisher is closed")
	}
	if p.current == nil {
		next, err := p.open()
		if err != nil {
			// Open errors are already free of credentials; keep a fixed message anyway.
			return errors.New("reconnect RabbitMQ publisher failed")
		}
		p.current = next
	}
	if err := p.current.Publish(ctx, envelope); err != nil {
		_ = p.current.Close()
		p.current = nil
		return err
	}
	return nil
}

// Close closes the current connection; later publishes fail.
func (p *recoveringPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	if p.current == nil {
		return nil
	}
	err := p.current.Close()
	p.current = nil
	return err
}
