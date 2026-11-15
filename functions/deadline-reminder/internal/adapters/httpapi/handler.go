// Package httpapi is the trigger endpoint. In k3d the PingSource posts a
// CloudEvent to it once a day; in Docker Compose it is called by hand (JT-61).
package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/app"
)

const (
	// MaxTriggerBody bounds the ignored CloudEvent body.
	MaxTriggerBody = 64 << 10
	// RunTimeout bounds one run, below the 300 s Knative request timeout.
	RunTimeout = 4 * time.Minute
)

// Runner is satisfied by app.Runner.
type Runner interface {
	Run(ctx context.Context) (app.Result, error)
}

// Handler runs one reminder pass per POST. Only 204 means every reminder was
// published; any failure is 503 so the trigger can be retried, which is safe
// because every reminder keeps its event_id.
type Handler struct {
	Runner Runner
	Logger *slog.Logger
	mu     sync.Mutex
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The PingSource data carries nothing we need; read it so the connection can be reused.
	if _, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, MaxTriggerBody)); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// One run at a time keeps the upstream load to one pass.
	if !h.mu.TryLock() {
		http.Error(w, "deadline reminder already running", http.StatusConflict)
		return
	}
	defer h.mu.Unlock()

	logger := h.Logger
	if id := r.Header.Get("Ce-Id"); id != "" {
		logger = logger.With("request_id", id)
	}
	ctx, cancel := context.WithTimeout(r.Context(), RunTimeout)
	defer cancel()
	result, err := h.Runner.Run(ctx)
	attrs := []any{"jobs", result.Jobs, "reminders", result.Reminders, "published", result.Published,
		"failed", result.Failed, "skipped", result.Skipped}
	if err != nil {
		// Only a fixed reason is logged; upstream errors and user IDs never are.
		logger.Error("deadline reminder failed", append(attrs, "reason", reason(err))...)
		http.Error(w, "deadline reminder failed", http.StatusServiceUnavailable)
		return
	}
	logger.Info("deadline reminders published", attrs...)
	w.WriteHeader(http.StatusNoContent)
}

func reason(err error) string {
	switch {
	case errors.Is(err, app.ErrJobs):
		return "closing jobs unavailable"
	case errors.Is(err, app.ErrSavers):
		return "job savers unavailable"
	case errors.Is(err, app.ErrPublish):
		return "reminder publish failed"
	default:
		return "unexpected error"
	}
}
