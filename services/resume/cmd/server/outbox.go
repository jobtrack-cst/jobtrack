package main

import (
	"context"
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// recoveringPublisher is owned by the single worker. Shutdown closes it only
// after joining the worker. No background reconnects or per-attempt goroutines.
type recoveringPublisher struct {
	current publisherResource
	open    func(string) (publisherResource, error)
	url     string
}

func (p *recoveringPublisher) Publish(ctx context.Context, event *eventsv1.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.current == nil {
		next, err := p.open(p.url)
		if err != nil {
			return errors.New("reconnect RabbitMQ publisher failed")
		}
		p.current = next
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.current.Publish(ctx, event); err != nil {
		if closeErr := p.Close(); closeErr != nil {
			return ports.ErrPublisherUnusable
		}
		return errors.New("RabbitMQ publication not confirmed")
	}
	return nil
}

func (p *recoveringPublisher) Close() error {
	if p.current == nil {
		return nil
	}
	old := p.current
	p.current = nil
	return old.Close()
}

// runWithDispatcher cancels and joins both fixed goroutines before resource
// owners close publisher and pool. Shared AMQP I/O may delay joining indefinitely.
func runWithDispatcher(ctx context.Context, worker, start func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	workers := make(chan error, 1)
	servers := make(chan error, 1)
	go func() { workers <- worker(ctx) }()
	go func() { servers <- start(ctx) }()
	var workerErr, serverErr error
	select {
	case workerErr = <-workers:
		cancel()
		serverErr = <-servers
	case serverErr = <-servers:
		cancel()
		workerErr = <-workers
	}
	return errors.Join(workerErr, serverErr)
}
