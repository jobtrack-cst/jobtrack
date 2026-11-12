package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
)

// stubOutbox satisfies the port in wiring tests that never call it.
type stubOutbox struct{ ports.OutboxRepository }

func TestRunWithDatabase_WiresOutboxRepository(t *testing.T) {
	cfg := config{databaseURL: "database-config", migrationsDir: t.TempDir()}
	started := false
	err := runWithDatabase(context.Background(), cfg, logging.New("test", io.Discard),
		func(context.Context, string) (*databaseHandle, error) {
			return &databaseHandle{
				readiness: readinessFunc(func(context.Context) error { return nil }),
				requests:  stubRequests{},
				outbox:    stubOutbox{},
				migrate:   func(context.Context, string) error { return nil },
				close:     func() {},
			}, nil
		}, func(_ context.Context, _ config, _ *slog.Logger, d deps) error {
			started = true
			if d.outbox == nil {
				t.Error("outbox repository was not wired")
			}
			return nil
		})
	if err != nil || !started {
		t.Fatalf("err=%v started=%v", err, started)
	}
}

func TestRunWithPublisher_PassesPublisherToRelay(t *testing.T) {
	var calls []string
	p := &startupPublisher{calls: &calls}
	cfg := config{rabbitMQURL: "broker-config"}
	err := runWithPublisher(context.Background(), cfg, logging.New("test", io.Discard), deps{},
		func(string) (publisherResource, error) { return p, nil },
		func(ctx context.Context, _ config, _ *slog.Logger, d deps) error {
			if d.publisher == nil {
				t.Fatal("no publisher was passed on for the outbox relay")
			}
			// startupPublisher records a publish as "unexpected publish".
			if err := d.publisher.Publish(ctx, nil); err != nil {
				t.Fatal(err)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"unexpected publish", "close RabbitMQ"}
	if len(calls) != 2 || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls=%v, want the opened publisher used and then closed: %v", calls, want)
	}
}
