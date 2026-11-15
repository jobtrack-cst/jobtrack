package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
)

// runWorkers starts both consumers on the broker URL from the config, each on
// its own queue with its own bindings (JT-60 and JT-81).
func TestConsumerConfigs_ApplicationAndHiring(t *testing.T) {
	cfgs := consumerConfigs(config{rabbitMQURL: "amqp://synthetic:synthetic@rabbitmq:5672/"},
		slog.New(slog.NewTextHandler(io.Discard, nil)), deps{})
	if len(cfgs) != 2 {
		t.Fatalf("got %d consumers, want 2", len(cfgs))
	}
	want := map[string][]string{
		"application-status.applications": {"application.submitted", "application.withdrawn"},
		"application-status.hiring":       {"review.completed", "interview.scheduled", "offer.made", "offer.accepted", "offer.declined"},
	}
	for _, c := range cfgs {
		bindings, ok := want[c.Queue]
		if !ok {
			t.Fatalf("unexpected queue %q", c.Queue)
		}
		if len(c.Bindings) != len(bindings) {
			t.Fatalf("%s bindings %v, want %v", c.Queue, c.Bindings, bindings)
		}
		for i := range bindings {
			if c.Bindings[i] != bindings[i] {
				t.Fatalf("%s bindings %v, want %v", c.Queue, c.Bindings, bindings)
			}
		}
		if c.URL != "amqp://synthetic:synthetic@rabbitmq:5672/" || c.Handler == nil || c.Logger == nil {
			t.Fatalf("%s is not fully configured", c.Queue)
		}
		delete(want, c.Queue)
	}
}

type oneEventOutbox struct {
	mu     sync.Mutex
	marked bool
}

func (o *oneEventOutbox) Pending(context.Context, int) ([]*eventsv1.EventEnvelope, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.marked {
		return nil, nil
	}
	return []*eventsv1.EventEnvelope{{EventId: "stored-event"}}, nil
}

func (o *oneEventOutbox) MarkPublished(context.Context, string, time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.marked = true
	return nil
}

type channelPublisher chan string

func (p channelPublisher) Publish(_ context.Context, e *eventsv1.EventEnvelope) error {
	p <- e.GetEventId()
	return nil
}

// runWorkers runs both consumers, the relay and the servers together, and on
// shutdown stops and joins all of them. The broker is unreachable here, so the
// consumers keep reconnecting without stopping the service.
func TestRunWorkers_RunsRelayAndStopsEverythingOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	published := make(channelPublisher, 10)
	done := make(chan error, 1)
	go func() {
		done <- runWorkers(ctx, config{rabbitMQURL: "amqp://synthetic:synthetic@127.0.0.1:1/"},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			deps{checker: app.NewHealth(), statuses: stubStatuses{}, outbox: &oneEventOutbox{}, publisher: published})
	}()
	select {
	case id := <-published:
		if id != "stored-event" {
			t.Fatalf("published %q, want the stored event", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relay did not publish the stored event")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWorkers returned %v after shutdown, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runWorkers did not stop after shutdown")
	}
}
