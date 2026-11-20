package main

import (
	"context"
	"errors"
)

// errConsumerStopped hides the consumer's own error, which could name the broker.
var errConsumerStopped = errors.New("audit event consumer stopped unexpectedly")

// runWithConsumer runs the event consumer next to the servers. When either
// stops, the other is cancelled, and both have returned before this does, so
// the consumer never outlives the database pool (JT-72, as in JT-70).
func runWithConsumer(ctx context.Context, consume, serve func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consume(runCtx) }()
	serveDone := make(chan error, 1)
	go func() { serveDone <- serve(runCtx) }()

	select {
	case err := <-serveDone:
		cancel()
		<-consumerDone
		return err
	case <-consumerDone:
		stoppedEarly := ctx.Err() == nil
		cancel()
		err := <-serveDone
		if stoppedEarly {
			return errors.Join(errConsumerStopped, err)
		}
		return err
	}
}
