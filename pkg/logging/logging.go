package logging

import (
	"io"
	"log/slog"
)

// New returns a JSON logger that includes the service name in each entry.
func New(service string, output io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, nil)).With("service", service)
}
