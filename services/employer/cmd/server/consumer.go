package main

import (
	"context"
	"errors"
)

// runWithConsumer runs the event consumer next to the servers. If the
// consumer fails (only an invalid configuration does), the servers stop too,
// so the service never runs without applying verification events. When the
// servers stop, the consumer is stopped and waited for.
func runWithConsumer(ctx context.Context, consume, serve func(context.Context) error) error {
	serveCtx, stopServe := context.WithCancel(ctx)
	defer stopServe()
	consumerCtx, stopConsumer := context.WithCancel(ctx)
	defer stopConsumer()
	consumerDone := make(chan error, 1)
	go func() {
		err := consume(consumerCtx)
		if err != nil {
			stopServe()
		}
		consumerDone <- err
	}()
	serveErr := serve(serveCtx)
	stopConsumer()
	if consumeErr := <-consumerDone; consumeErr != nil {
		return errors.Join(errors.New("event consumer stopped"), consumeErr, serveErr)
	}
	return serveErr
}
