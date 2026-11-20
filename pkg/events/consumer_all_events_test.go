package events

import (
	"context"
	"io"
	"log/slog"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// JT-72: audit-log needs every event, so the literal "#" is the one topic
// wildcard allowed; partial wildcards would hide which events a queue gets.
func TestConsumerConfig_ValidateAllowsOnlyTheAllEventsWildcard(t *testing.T) {
	handler := func(context.Context, *eventsv1.EventEnvelope) error { return nil }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tt := range []struct {
		binding string
		valid   bool
	}{
		{"#", true},
		{AllEventsBinding, true},
		{"job.closed", true},
		{"job_alert.matched", true},
		{"*", false},
		{"job.*", false},
		{"job.#", false},
		{"#.closed", false},
		{"##", false},
		{" #", false},
		{"# ", false},
		{"", false},
	} {
		t.Run(tt.binding, func(t *testing.T) {
			cfg := ConsumerConfig{URL: "amqp://broker", Queue: "audit-log.events",
				Bindings: []string{tt.binding}, Handler: handler, Logger: logger}
			err := cfg.validate()
			if tt.valid && err != nil {
				t.Fatalf("binding %q refused: %v", tt.binding, err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("binding %q accepted", tt.binding)
			}
		})
	}
}
