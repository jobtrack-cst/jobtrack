package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jobtrack-cst/jobtrack/functions/email-sender/internal/sender"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
)

func main() {
	if err := run(); err != nil {
		logging.New("email-sender", os.Stdout).Error("function stopped")
		os.Exit(1)
	}
}
func run() error {
	smtpPort, err := sender.NewSMTP(os.Getenv("SMTP_ADDR"), os.Getenv("SMTP_FROM"))
	if err != nil {
		return err
	}
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("invalid HTTP_PORT")
	}
	logger := logging.New("email-sender", os.Stdout)
	mux := http.NewServeMux()
	mux.Handle("/send-email", sender.Handler{Delivery: smtpPort, Logger: logger})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("jobtrack_email_sender_up 1\n"))
	})
	server := sender.NewServer(":"+port, mux)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	select {
	case err := <-failures:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), sender.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdown)
}
