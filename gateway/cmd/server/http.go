package main

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/middleware"
)

// newHandler serves /healthz and /metrics without a token and sends every other
// request through Authenticate to grpc-gateway.
func newHandler(api http.Handler, verifier middleware.TokenVerifier, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		// The gateway has no database or broker, so it is ready whenever it serves.
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "# HELP jobtrack_service_ready Whether the service readiness check succeeds.\n"+
			"# TYPE jobtrack_service_ready gauge\n"+
			"jobtrack_service_ready 1\n")
	})
	mux.Handle("/", middleware.Authenticate(verifier, logger, api))
	return mux
}
