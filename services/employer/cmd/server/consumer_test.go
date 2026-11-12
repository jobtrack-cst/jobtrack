package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunWithConsumer_StopsConsumerWhenServersStop(t *testing.T) {
	consumerStopped := make(chan struct{})
	err := runWithConsumer(context.Background(),
		func(ctx context.Context) error {
			<-ctx.Done()
			close(consumerStopped)
			return nil
		},
		func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-consumerStopped:
	default:
		t.Fatal("runWithConsumer returned before the consumer stopped")
	}
}

func TestRunWithConsumer_ReturnsServerError(t *testing.T) {
	errBind := errors.New("bind failed")
	err := runWithConsumer(context.Background(),
		func(ctx context.Context) error { <-ctx.Done(); return nil },
		func(context.Context) error { return errBind })
	if !errors.Is(err, errBind) {
		t.Fatalf("err=%v, want %v", err, errBind)
	}
}

func TestRunWithConsumer_ConsumerFailureStopsServers(t *testing.T) {
	errConfig := errors.New("queue must be named <service>.<purpose>")
	done := make(chan error, 1)
	go func() {
		done <- runWithConsumer(context.Background(),
			func(context.Context) error { return errConfig },
			func(ctx context.Context) error { <-ctx.Done(); return nil })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, errConfig) || !strings.Contains(err.Error(), "event consumer stopped") {
			t.Fatalf("err=%v, want the consumer error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("servers kept running after the consumer failed")
	}
}

func TestRunWithConsumer_ShutdownStopsBoth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithConsumer(ctx,
			func(ctx context.Context) error { <-ctx.Done(); return nil },
			func(ctx context.Context) error { <-ctx.Done(); return nil })
	}()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not stop the servers and consumer")
	}
}
