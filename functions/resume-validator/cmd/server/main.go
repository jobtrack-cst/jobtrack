package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := logging.New("resume-validator", os.Stdout)
	if err := run(); err != nil {
		logger.Error("function stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	if os.Getenv("SERVICE_NAME") != "resume-validator" || !validator.Required(os.Getenv("RESUME_ADDR")) || !validator.Required(os.Getenv("RABBITMQ_URL")) {
		return errors.New("SERVICE_NAME=resume-validator, RESUME_ADDR and RABBITMQ_URL are required")
	}
	interceptor, err := grpcclient.InternalUnaryClientInterceptor(os.Getenv("INTERNAL_TOKEN"), os.Getenv("SERVICE_NAME"))
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(os.Getenv("RESUME_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(interceptor))
	if err != nil {
		return err
	}
	defer conn.Close()
	logger := logging.New("resume-validator", os.Stdout)
	mux := http.NewServeMux()
	mux.Handle("/", validator.Handler{Files: resumev1.NewResumeInternalServiceClient(conn), Events: validator.BrokerPublisher{URL: os.Getenv("RABBITMQ_URL")}, Logger: logger})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("jobtrack_resume_validator_up 1\n"))
	})
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 12 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	select {
	case err := <-failures:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
