package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// JT-114: the application consumer runs beside the servers, and either one
// stopping stops the other.
func TestRunWithConsumer_StopsTogether(t *testing.T) {
	errConsumer := errors.New("consumer configuration rejected")
	errServers := errors.New("bind failed")
	waitForCancel := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("not canceled")
		}
	}
	for _, tt := range []struct {
		name           string
		consume, start func(context.Context) error
		want           error
	}{
		{
			name:    "consumer failure stops the servers",
			consume: func(context.Context) error { return errConsumer },
			start:   waitForCancel,
			want:    errConsumer,
		},
		{
			name:    "server failure stops the consumer",
			consume: waitForCancel,
			start:   func(context.Context) error { return errServers },
			want:    errServers,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := runWithConsumer(context.Background(), tt.consume, tt.start)
			// The other side returned nil after its cancel, so only want remains.
			if !errors.Is(err, tt.want) || err.Error() != tt.want.Error() {
				t.Fatalf("err=%v, want only %v", err, tt.want)
			}
		})
	}
	t.Run("shutdown stops both without an error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{}, 2)
		wait := func(ctx context.Context) error {
			started <- struct{}{}
			return waitForCancel(ctx)
		}
		done := make(chan error, 1)
		go func() { done <- runWithConsumer(ctx, wait, wait) }()
		<-started
		<-started
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("err=%v, want nil", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("runWithConsumer did not stop")
		}
	})
}
