package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func blockUntilDone(stopped *atomic.Bool) func(context.Context) error {
	return func(ctx context.Context) error {
		<-ctx.Done()
		stopped.Store(true)
		return nil
	}
}

func runConsumerForTest(t *testing.T, ctx context.Context, consume, serve func(context.Context) error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- runWithConsumer(ctx, consume, serve) }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("runWithConsumer did not return")
		return nil
	}
}

func TestRunWithConsumer_ShutdownStopsBoth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var consumerStopped, serverStopped atomic.Bool
	time.AfterFunc(50*time.Millisecond, cancel)
	err := runConsumerForTest(t, ctx, blockUntilDone(&consumerStopped), blockUntilDone(&serverStopped))
	require.NoError(t, err)
	require.True(t, consumerStopped.Load())
	require.True(t, serverStopped.Load())
}

func TestRunWithConsumer_ServerFailureStopsConsumerFirst(t *testing.T) {
	errBind := errors.New("bind gRPC listener failed")
	var consumerStopped atomic.Bool
	err := runConsumerForTest(t, context.Background(), blockUntilDone(&consumerStopped),
		func(context.Context) error { return errBind })
	require.ErrorIs(t, err, errBind)
	require.True(t, consumerStopped.Load(), "the consumer has stopped before the pool can close")
}

func TestRunWithConsumer_ConsumerStopStopsServers(t *testing.T) {
	var serverStopped atomic.Bool
	err := runConsumerForTest(t, context.Background(),
		func(context.Context) error { return errors.New("private broker detail") }, blockUntilDone(&serverStopped))
	require.ErrorIs(t, err, errConsumerStopped)
	require.NotContains(t, err.Error(), "private")
	require.True(t, serverStopped.Load())
}

func TestRunWithConsumer_ConsumerReturningNilEarlyIsAnError(t *testing.T) {
	var serverStopped atomic.Bool
	err := runConsumerForTest(t, context.Background(),
		func(context.Context) error { return nil }, blockUntilDone(&serverStopped))
	require.ErrorIs(t, err, errConsumerStopped)
	require.True(t, serverStopped.Load())
}
