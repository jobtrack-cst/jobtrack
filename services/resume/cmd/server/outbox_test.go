package main

import (
	"context"
	"errors"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
	"github.com/stretchr/testify/require"
)

type recoveryStub struct {
	publish func(context.Context, *eventsv1.EventEnvelope) error
	close   func() error
}

func (p recoveryStub) Publish(ctx context.Context, e *eventsv1.EventEnvelope) error {
	return p.publish(ctx, e)
}
func (p recoveryStub) Close() error { return p.close() }

func TestRecoveringPublisher_ReconnectsOnlyAfterClosingFailedSession(t *testing.T) {
	for _, scenario := range []string{"recover", "reconnect fails", "close fails", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			var calls []string
			p := &recoveringPublisher{url: "private-url", current: recoveryStub{publish: func(context.Context, *eventsv1.EventEnvelope) error {
				calls = append(calls, "publish old")
				return errors.New("private-error")
			}, close: func() error {
				calls = append(calls, "close old")
				if scenario == "close fails" {
					return errors.New("private-error")
				}
				return nil
			}}, open: func(url string) (publisherResource, error) {
				require.Equal(t, "private-url", url)
				calls = append(calls, "open new")
				if scenario == "reconnect fails" {
					return nil, errors.New("private-error")
				}
				return recoveryStub{publish: func(context.Context, *eventsv1.EventEnvelope) error { calls = append(calls, "publish new"); return nil }, close: func() error { calls = append(calls, "close new"); return nil }}, nil
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			err := p.Publish(ctx, &eventsv1.EventEnvelope{})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-")
			if scenario == "close fails" {
				require.ErrorIs(t, err, ports.ErrPublisherUnusable)
				require.Equal(t, []string{"publish old", "close old"}, calls)
				return
			}
			if scenario == "canceled" {
				require.Empty(t, calls)
				require.NoError(t, p.Close())
				return
			}
			err = p.Publish(ctx, &eventsv1.EventEnvelope{})
			if scenario == "reconnect fails" {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-")
				require.Equal(t, []string{"publish old", "close old", "open new"}, calls)
			} else {
				require.NoError(t, err)
				require.NoError(t, p.Close())
				require.NoError(t, p.Close())
				require.Equal(t, []string{"publish old", "close old", "open new", "publish new", "close new"}, calls)
			}
		})
	}
}

func TestRunWithDispatcher_JoinsBothBeforeReturning(t *testing.T) {
	for _, fails := range []string{"worker", "servers", "cancel"} {
		t.Run(fails, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			workerStarted := make(chan struct{})
			serverStarted := make(chan struct{})
			workerStopped := make(chan struct{})
			serverStopped := make(chan struct{})
			failure := errors.New("controlled failure")
			done := make(chan error, 1)
			go func() {
				done <- runWithDispatcher(ctx, func(ctx context.Context) error {
					close(workerStarted)
					defer close(workerStopped)
					<-serverStarted
					if fails == "worker" {
						return failure
					}
					<-ctx.Done()
					return nil
				}, func(ctx context.Context) error {
					close(serverStarted)
					defer close(serverStopped)
					<-workerStarted
					if fails == "servers" {
						return failure
					}
					<-ctx.Done()
					return nil
				})
			}()
			<-workerStarted
			<-serverStarted
			if fails == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				if fails == "cancel" {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, failure)
				}
			case <-time.After(time.Second):
				t.Fatal("lifecycle did not join")
			}
			select {
			case <-workerStopped:
			default:
				t.Fatal("worker still running")
			}
			select {
			case <-serverStopped:
			default:
				t.Fatal("servers still running")
			}
		})
	}
}

func TestRunWithDispatcher_JoinsConsumerDispatcherAndListeners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 3)
	stopped := make(chan struct{}, 3)
	run := func(c context.Context) error {
		started <- struct{}{}
		defer func() { stopped <- struct{}{} }()
		<-c.Done()
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- runWithDispatcher(ctx, run, func(c context.Context) error { return runWithDispatcher(c, run, run) })
	}()
	for i := 0; i < 3; i++ {
		<-started
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("service workers did not join")
	}
	require.Len(t, stopped, 3)
}
