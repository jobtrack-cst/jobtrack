// Command server is the deadline-reminder function (JT-82): one POST finds the
// jobs closing within 48 hours and publishes one deadline.reminder per saver.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/adapters/httpapi"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/app"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/ports"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// serviceName is also the x-caller-service value the job-posting and
// saved-jobs allowlists expect (AGENTS.md Section 6).
const serviceName = "deadline-reminder"

const (
	// WriteTimeout lets a full run finish before the response is written.
	WriteTimeout = httpapi.RunTimeout + 15*time.Second
	// ShutdownTimeout drains a request in progress on SIGTERM.
	ShutdownTimeout = 30 * time.Second
)

type config struct {
	httpPort       string
	jobPostingAddr string
	savedJobsAddr  string
	// Never logged: both contain credentials.
	rabbitMQURL   string
	internalToken string
}

func main() {
	logger := logging.New(serviceName, os.Stdout)
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logger.Error("invalid startup configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("function stopped", "error", err)
		os.Exit(1)
	}
}

// loadConfig fails on any missing or invalid setting. Messages never contain
// the values.
func loadConfig(getenv func(string) string) (config, error) {
	if getenv("SERVICE_NAME") != serviceName {
		return config{}, errors.New("SERVICE_NAME must be deadline-reminder")
	}
	cfg := config{
		httpPort:       getenv("HTTP_PORT"),
		jobPostingAddr: strings.TrimSpace(getenv("JOB_POSTING_ADDR")),
		savedJobsAddr:  strings.TrimSpace(getenv("SAVED_JOBS_ADDR")),
		rabbitMQURL:    strings.TrimSpace(getenv("RABBITMQ_URL")),
		internalToken:  getenv("INTERNAL_TOKEN"),
	}
	if cfg.httpPort == "" {
		cfg.httpPort = "8080"
	}
	if n, err := strconv.Atoi(cfg.httpPort); err != nil || n < 1 || n > 65535 {
		return config{}, errors.New("HTTP_PORT must be an integer from 1 to 65535")
	}
	if cfg.jobPostingAddr == "" {
		return config{}, errors.New("JOB_POSTING_ADDR is required")
	}
	if cfg.savedJobsAddr == "" {
		return config{}, errors.New("SAVED_JOBS_ADDR is required")
	}
	if u, err := url.Parse(cfg.rabbitMQURL); err != nil || (u.Scheme != "amqp" && u.Scheme != "amqps") || u.Host == "" {
		return config{}, errors.New("RABBITMQ_URL must be an amqp:// or amqps:// URL")
	}
	if !authctx.ValidInternalToken(cfg.internalToken) {
		return config{}, errors.New("INTERNAL_TOKEN must contain nonempty visible ASCII without whitespace")
	}
	return cfg, nil
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	jobPostingConn, err := dialInternal(cfg.jobPostingAddr, cfg.internalToken)
	if err != nil {
		return err
	}
	defer jobPostingConn.Close()
	savedJobsConn, err := dialInternal(cfg.savedJobsAddr, cfg.internalToken)
	if err != nil {
		return err
	}
	defer savedJobsConn.Close()
	runner := publishingRunner{
		jobs:   clients.JobPosting{API: jobpostingv1.NewJobPostingInternalServiceClient(jobPostingConn)},
		savers: clients.SavedJobs{API: savedjobsv1.NewSavedJobsInternalServiceClient(savedJobsConn)},
		open:   func(url string) (publisherResource, error) { return events.NewPublisher(url) },
		url:    cfg.rabbitMQURL,
		now:    time.Now,
	}
	server := newServer(":"+cfg.httpPort, newMux(&httpapi.Handler{Runner: runner, Logger: logger}))
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	logger.Info("function starting", "http_address", server.Addr)
	select {
	case err := <-failures:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdown)
}

type publisherResource interface {
	ports.Publisher
	Close() error
}

// publishingRunner opens a broker connection for each run and closes it
// afterwards. A daily function would otherwise depend on a connection left
// open for 24 hours, which a broker restart could have broken.
type publishingRunner struct {
	jobs   ports.Jobs
	savers ports.Savers
	open   func(url string) (publisherResource, error)
	url    string
	now    func() time.Time
}

func (r publishingRunner) Run(ctx context.Context) (app.Result, error) {
	publisher, err := r.open(r.url)
	if err != nil {
		// The URL holds the broker password, so the error is not wrapped.
		return app.Result{}, fmt.Errorf("%w: broker unavailable", app.ErrPublish)
	}
	defer publisher.Close()
	runner := app.Runner{Jobs: r.jobs, Savers: r.savers, Publisher: publisher, Now: r.now}
	return runner.Run(ctx)
}

// dialInternal gives each backend its own interceptor, so each has its own
// circuit breaker (JT-31). The connection is lazy: startup does not wait for
// job-posting or saved-jobs.
func dialInternal(addr, token string) (*grpc.ClientConn, error) {
	interceptor, err := grpcclient.InternalUnaryClientInterceptor(token, serviceName)
	if err != nil {
		return nil, errors.New("INTERNAL_TOKEN is invalid")
	}
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(interceptor))
}

// newMux serves the trigger on "/" only, plus health and metrics.
func newMux(trigger http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/{$}", trigger)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("jobtrack_deadline_reminder_up 1\n"))
	})
	return mux
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
}
