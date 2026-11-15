// Package httpapi is the trigger endpoint. In k3d the PingSource posts a
// CloudEvent to it once a week; in Docker Compose it is called by hand.
package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/app"
)

const (
	// MaxTriggerBody bounds the ignored CloudEvent body.
	MaxTriggerBody = 64 << 10
	// RunTimeout bounds one run: two reads plus up to 30 s per email.
	RunTimeout = 4 * time.Minute
)

// Runner is satisfied by app.Runner.
type Runner interface {
	Run(ctx context.Context) (app.Result, error)
}

// Handler runs one summary per POST. Only 204 means every admin got the
// email; any failure is 503 so the trigger can be retried.
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
	// Two overlapping runs would email every admin twice.
	if !h.mu.TryLock() {
		http.Error(w, "weekly summary already running", http.StatusConflict)
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
	attrs := []any{"admins", result.Admins, "sent", result.Sent, "failed", result.Failed}
	if err != nil {
		// Only a fixed reason is logged; upstream errors and emails never are.
		logger.Error("weekly summary failed", append(attrs, "reason", reason(err))...)
		http.Error(w, "weekly summary failed", http.StatusServiceUnavailable)
		return
	}
	logger.Info("weekly summary sent", attrs...)
	w.WriteHeader(http.StatusNoContent)
}

func reason(err error) string {
	switch {
	case errors.Is(err, app.ErrStats):
		return "statistics unavailable"
	case errors.Is(err, app.ErrAdmins):
		return "admin list unavailable"
	case errors.Is(err, app.ErrSend):
		return "email delivery failed"
	default:
		return "unexpected error"
	}
}
