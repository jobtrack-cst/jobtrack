// Command server is the weekly-summary function (JT-86): one POST sends the
// statistics of the last 7 days to every admin.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/emailsender"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/httpapi"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/app"
	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// serviceName is also the x-caller-service value the analytics and auth
// allowlists expect (AGENTS.md Section 6).
const serviceName = "weekly-summary"

const (
	// WriteTimeout lets a full run finish before the response is written.
	WriteTimeout = httpapi.RunTimeout + 15*time.Second
	// ShutdownTimeout drains a request in progress on SIGTERM.
	ShutdownTimeout = 30 * time.Second
)

type config struct {
	httpPort       string
	analyticsAddr  string
	authAddr       string
	emailSenderURL string
	// Never logged.
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
		return config{}, errors.New("SERVICE_NAME must be weekly-summary")
	}
	cfg := config{
		httpPort:       getenv("HTTP_PORT"),
		analyticsAddr:  strings.TrimSpace(getenv("ANALYTICS_ADDR")),
		authAddr:       strings.TrimSpace(getenv("AUTH_ADDR")),
		emailSenderURL: getenv("EMAIL_SENDER_URL"),
		internalToken:  getenv("INTERNAL_TOKEN"),
	}
	if cfg.httpPort == "" {
		cfg.httpPort = "8080"
	}
	if n, err := strconv.Atoi(cfg.httpPort); err != nil || n < 1 || n > 65535 {
		return config{}, errors.New("HTTP_PORT must be an integer from 1 to 65535")
	}
	if cfg.analyticsAddr == "" {
		return config{}, errors.New("ANALYTICS_ADDR is required")
	}
	if cfg.authAddr == "" {
		return config{}, errors.New("AUTH_ADDR is required")
	}
	if _, err := emailsender.New(cfg.emailSenderURL); err != nil {
		return config{}, err
	}
	if !authctx.ValidInternalToken(cfg.internalToken) {
		return config{}, errors.New("INTERNAL_TOKEN must contain nonempty visible ASCII without whitespace")
	}
	return cfg, nil
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	analyticsConn, err := dialInternal(cfg.analyticsAddr, cfg.internalToken)
	if err != nil {
		return err
	}
	defer analyticsConn.Close()
	authConn, err := dialInternal(cfg.authAddr, cfg.internalToken)
	if err != nil {
		return err
	}
	defer authConn.Close()
	mailer, err := emailsender.New(cfg.emailSenderURL)
	if err != nil {
		return err
	}
	runner := app.Runner{
		Stats:  clients.Analytics{API: analyticsv1.NewAnalyticsInternalServiceClient(analyticsConn)},
		Admins: clients.Auth{API: authv1.NewAuthInternalServiceClient(authConn)},
		Mailer: mailer,
		Now:    time.Now,
		Logger: logger,
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

// dialInternal gives each backend its own interceptor, so each has its own
// circuit breaker (JT-31). The connection is lazy: startup does not wait for
// analytics or auth.
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
		_, _ = w.Write([]byte("jobtrack_weekly_summary_up 1\n"))
	})
	return mux
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
}
