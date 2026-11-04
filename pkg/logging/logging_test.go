package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
)

func TestNew_EmitsJSONWithServiceMessageAndLevel(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
	}{
		{name: "info", level: slog.LevelInfo},
		{name: "warn", level: slog.LevelWarn},
		{name: "error", level: slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := logging.New("application", &output)
			message := "processing \"application\"\nstarted"
			logger.Log(context.Background(), tt.level, message)

			var entry map[string]any
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatalf("log output is not valid JSON: %v; output: %q", err, output.String())
			}
			for key, want := range map[string]any{
				"service": "application", "msg": message, "level": tt.level.String(),
			} {
				if got := entry[key]; got != want {
					t.Errorf("%s = %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestNew_PreservesContextualAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New("application", &output).With("request_id", "request-123")
	logger.With("event_id", "event-456").Info("application submitted", "attempt", 2)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	for key, want := range map[string]any{
		"service": "application", "request_id": "request-123", "event_id": "event-456",
		"attempt": float64(2), "msg": "application submitted", "level": "INFO",
	} {
		if got := entry[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}
