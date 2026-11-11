package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := logging.New("rabbitmq-forwarder", os.Stdout)
	c := forwarder.Config{Broker: os.Getenv("RABBITMQ_URL"), Sink: os.Getenv("SINK_URL"), Function: os.Getenv("FUNCTION_NAME"), Bindings: strings.Split(os.Getenv("EVENT_TYPES"), ",")}
	if err := c.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# Forwarder process is running.\njobtrack_forwarder_up 1\n"))
	})
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed")
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
	}()
	for ctx.Err() == nil {
		if err := forwarder.Run(ctx, c, logger); err != nil {
			logger.Warn("forwarder session ended", "error", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}
