package main

import (
	"io"
	"net/http"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

func healthHandler(checker ports.HealthChecker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		health, err := checker.CheckHealth(r.Context())
		if err != nil || !health.IsHealthy() {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("GET /metrics", metricsHandler(checker))
	return mux
}

func metricsHandler(checker ports.HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health, err := checker.CheckHealth(r.Context())
		ready := "0"
		if err == nil && health.IsHealthy() {
			ready = "1"
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "# HELP jobtrack_service_ready Whether the service readiness check succeeds.\n"+
			"# TYPE jobtrack_service_ready gauge\n"+
			"jobtrack_service_ready "+ready+"\n")
	}
}
