package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
)

// outboxRelayInterval is how long the relay waits when the outbox is empty or
// publishing failed. A decision is published within about this time.
const outboxRelayInterval = time.Second

type outboxPublisher interface {
	PublishPending(context.Context) (int, error)
}

// runWithRelay runs the outbox relay next to the servers and stops it when
// they stop, so no publish is left running after shutdown.
func runWithRelay(ctx context.Context, logger *slog.Logger, relay outboxPublisher, interval time.Duration, serve func(context.Context) error) error {
	relayCtx, stopRelay := context.WithCancel(ctx)
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		relayLoop(relayCtx, logger, relay, interval)
	}()
	err := serve(ctx)
	stopRelay()
	<-relayDone
	return err
}

// relayLoop publishes pending outbox events until ctx ends. A full batch means
// more may be waiting, so it reads again at once; otherwise it waits interval.
func relayLoop(ctx context.Context, logger *slog.Logger, relay outboxPublisher, interval time.Duration) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		n, err := relay.PublishPending(ctx)
		if n > 0 {
			logger.Info("outbox events published", "count", n)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("publish outbox events failed; will retry", "error", err)
		}
		next := interval
		if err == nil && n == app.OutboxBatchSize {
			next = 0
		}
		timer.Reset(next)
	}
}
