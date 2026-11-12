package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
)

// fakeRelay returns the queued results in order, then (0, nil).
type fakeRelay struct {
	mu      sync.Mutex
	calls   int
	results []relayResult
	called  chan struct{}
}

type relayResult struct {
	n   int
	err error
}

func newFakeRelay(results ...relayResult) *fakeRelay {
	return &fakeRelay{results: results, called: make(chan struct{}, 100)}
}

func (f *fakeRelay) PublishPending(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.called <- struct{}{}
	if len(f.results) == 0 {
		return 0, nil
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.n, r.err
}

func (f *fakeRelay) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func waitCalls(t *testing.T, f *fakeRelay, n int) {
	t.Helper()
	for range n {
		select {
		case <-f.called:
		case <-time.After(5 * time.Second):
			t.Fatalf("relay was called %d times, want at least %d", f.callCount(), n)
		}
	}
}

func TestRunWithRelay_PublishesWhileServingAndStopsAfter(t *testing.T) {
	relay := newFakeRelay()
	var logs bytes.Buffer
	err := runWithRelay(context.Background(), logging.New("test", &logs), relay, time.Millisecond,
		func(context.Context) error {
			waitCalls(t, relay, 3)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	after := relay.callCount()
	time.Sleep(20 * time.Millisecond)
	if relay.callCount() != after {
		t.Fatal("relay kept running after the servers stopped")
	}
}

func TestRunWithRelay_ReturnsServerErrorAndStopsRelay(t *testing.T) {
	relay := newFakeRelay()
	errBind := errors.New("bind failed")
	err := runWithRelay(context.Background(), logging.New("test", &bytes.Buffer{}), relay, time.Hour,
		func(context.Context) error { return errBind })
	if !errors.Is(err, errBind) {
		t.Fatalf("err=%v, want %v", err, errBind)
	}
}

func TestRelayLoop_LogsFailureAndRetries(t *testing.T) {
	relay := newFakeRelay(relayResult{err: errors.New("broker unavailable")}, relayResult{n: 2})
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); relayLoop(ctx, logging.New("test", &logs), relay, time.Millisecond) }()
	waitCalls(t, relay, 3)
	cancel()
	<-done
	out := logs.String()
	if !strings.Contains(out, "will retry") || !strings.Contains(out, "outbox events published") {
		t.Fatalf("missing retry or published log: %s", out)
	}
}

func TestRelayLoop_FullBatchReadsAgainWithoutWaiting(t *testing.T) {
	relay := newFakeRelay(relayResult{n: app.OutboxBatchSize})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	// An hour interval: the second call only happens quickly if a full batch skips the wait.
	go func() { defer close(done); relayLoop(ctx, logging.New("test", &bytes.Buffer{}), relay, time.Hour) }()
	waitCalls(t, relay, 2)
	cancel()
	<-done
}

func TestRelayLoop_StopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		relayLoop(ctx, logging.New("test", &bytes.Buffer{}), newFakeRelay(), time.Hour)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("relay loop did not stop")
	}
}
