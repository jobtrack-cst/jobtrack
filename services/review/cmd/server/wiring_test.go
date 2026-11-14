package main

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

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

type noRecorder struct{}

func (noRecorder) RecordApplication(context.Context, string, domain.ApplicationParticipants) error {
	return nil
}

type recordingConnection struct {
	mu        sync.Mutex
	published chan string
	closed    int
}

func (c *recordingConnection) Publish(_ context.Context, e *eventsv1.EventEnvelope) error {
	c.published <- e.GetEventId()
	return nil
}

func (c *recordingConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed++
	return nil
}

// With the outbox and recorder wired, the relay publishes stored events through
// the opened connection while the servers run, and the connection is closed
// after shutdown. The consumer's broker is unreachable here; it keeps
// reconnecting until shutdown without stopping the servers.
func TestRunWithPublisher_RunsRelayAndConsumerWhenWired(t *testing.T) {
	conn := &recordingConnection{published: make(chan string, 10)}
	opened := 0
	cfg := config{rabbitMQURL: "amqp://synthetic:synthetic@127.0.0.1:1/"}
	err := runWithPublisher(context.Background(), cfg, logging.New("test", &bytes.Buffer{}),
		deps{outbox: &oneEventOutbox{}, recorder: noRecorder{}},
		func(string) (publisherResource, error) { opened++; return conn, nil },
		func(context.Context, config, *slog.Logger, deps) error {
			select {
			case id := <-conn.published:
				if id != "stored-event" {
					t.Errorf("published %q, want the stored event", id)
				}
			case <-time.After(5 * time.Second):
				t.Error("relay did not publish the stored event")
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 {
		t.Fatalf("opened %d connections, want 1", opened)
	}
	if conn.closed == 0 {
		t.Fatal("RabbitMQ connection not closed after shutdown")
	}
}
