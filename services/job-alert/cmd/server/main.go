package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	categoryv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	jobalertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/clients"
	grpcadapter "github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	categoryTagAddr string
	internalToken   string
	serviceName     string
	grpcPort        int
	httpPort        int
	databaseURL     string
	migrationsDir   string
	rabbitMQURL     string
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logging.New("job-alert", os.Stdout).Error("invalid startup configuration", "error", err)
		os.Exit(1)
	}
	logger := logging.New(cfg.serviceName, os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{serviceName: getenv("SERVICE_NAME")}
	if !regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`).MatchString(cfg.serviceName) {
		return config{}, errors.New("SERVICE_NAME must be a nonempty kebab-case name")
	}
	for _, port := range []struct {
		name     string
		fallback int
		target   *int
	}{
		{"GRPC_PORT", 9090, &cfg.grpcPort}, {"HTTP_PORT", 8080, &cfg.httpPort},
	} {
		value := getenv(port.name)
		if value == "" {
			*port.target = port.fallback
			continue
		}
		number, err := strconv.Atoi(value)
		if err != nil || number < 1 || number > 65535 {
			return config{}, fmt.Errorf("%s must be an integer from 1 to 65535", port.name)
		}
		*port.target = number
	}
	if cfg.grpcPort == cfg.httpPort {
		return config{}, errors.New("GRPC_PORT and HTTP_PORT must differ")
	}
	cfg.databaseURL = getenv("DATABASE_URL")
	if strings.TrimSpace(cfg.databaseURL) == "" {
		return config{}, errors.New("DATABASE_URL is required")
	}
	// Start from the repository root; tests can inject another migration directory.
	cfg.migrationsDir = "services/job-alert/migrations"
	cfg.rabbitMQURL = getenv("RABBITMQ_URL")
	if strings.TrimSpace(cfg.rabbitMQURL) == "" {
		return config{}, errors.New("RABBITMQ_URL is required")
	}
	if cfg.serviceName != "job-alert" {
		return config{}, errors.New("SERVICE_NAME must be job-alert")
	}
	cfg.categoryTagAddr = getenv("CATEGORY_TAG_ADDR")
	host, port, err := net.SplitHostPort(cfg.categoryTagAddr)
	number, portErr := strconv.Atoi(port)
	if err != nil || !validCategoryHost(host) || portErr != nil || number < 1 || number > 65535 {
		return config{}, errors.New("CATEGORY_TAG_ADDR must be host:port")
	}
	cfg.internalToken = getenv("INTERNAL_TOKEN")
	if _, err := grpcclient.InternalUnaryClientInterceptor(cfg.internalToken, cfg.serviceName); err != nil {
		return config{}, errors.New("INTERNAL_TOKEN is invalid")
	}
	broker, err := url.Parse(cfg.rabbitMQURL)
	if err != nil || broker.Host == "" || (broker.Scheme != "amqp" && broker.Scheme != "amqps") {
		return config{}, errors.New("RABBITMQ_URL must be an AMQP URL")
	}
	return cfg, nil
}

var categoryHostLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

func validCategoryHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || !categoryHostLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	return runWithDatabase(ctx, cfg, logger, openDatabase, func(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker, repo ports.AlertRepository, jobs ports.JobRepository, outbox ports.OutboxRepository) error {
		interceptor, err := grpcclient.InternalUnaryClientInterceptor(cfg.internalToken, cfg.serviceName)
		if err != nil {
			return errors.New("invalid internal client configuration")
		}
		conn, err := grpc.NewClient(cfg.categoryTagAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(interceptor))
		if err != nil {
			return errors.New("initialize category-tag client failed")
		}
		defer conn.Close()
		alerts := app.NewAlerts(repo, clients.NewNames(categoryv1.NewCategoryTagInternalServiceClient(conn)), nil)
		return runWithPublisher(ctx, cfg, logger, checker, func(url string) (publisherResource, error) { return rabbitmq.NewPublisher(url) }, func(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker, publisher ports.EventPublisher) error {
			recorder := app.NewJobs(jobs, wallClock{})
			relay := app.NewOutboxRelay(outbox, publisher, wallClock{})
			workerCtx, stop := context.WithCancel(ctx)
			consumerDone := make(chan error, 1)
			go func() {
				err := events.Consume(workerCtx, rabbitmq.JobConsumerConfig(cfg.rabbitMQURL, logger, recorder))
				consumerDone <- err
				stop()
			}()
			err = runWithRelay(workerCtx, logger, relay, outboxRelayInterval, func(ctx context.Context) error { return runServers(ctx, cfg, logger, checker, alerts) })
			stop()
			consumerErr := <-consumerDone
			if consumerErr != nil && !errors.Is(consumerErr, context.Canceled) {
				return errors.Join(err, errors.New("job consumer stopped"))
			}
			return err
		})
	})
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now().UTC() }

type publisherResource interface {
	ports.EventPublisher
	Close() error
}

func runWithPublisher(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker,
	open func(string) (publisherResource, error),
	start func(context.Context, config, *slog.Logger, ports.HealthChecker, ports.EventPublisher) error,
) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.rabbitMQURL) == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	first, err := open(cfg.rabbitMQURL)
	if err != nil {
		return errors.New("initialize RabbitMQ publisher failed")
	}
	publisher := newRecoveringPublisher(first, func() (publisherResource, error) { return open(cfg.rabbitMQURL) })
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			err = errors.Join(err, errors.New("close RabbitMQ publisher failed"))
		}
	}()
	return start(ctx, cfg, logger, checker, publisher)
}

// databaseHandle holds operations on one owned pool, without creating another pool manager.
type databaseHandle struct {
	alerts    ports.AlertRepository
	jobs      ports.JobRepository
	outbox    ports.OutboxRepository
	readiness ports.Readiness
	migrate   func(context.Context, string) error
	close     func()
}

func openDatabase(ctx context.Context, url string) (*databaseHandle, error) {
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		return nil, err
	}
	return &databaseHandle{
		alerts:    postgres.NewAlerts(pool),
		jobs:      postgres.NewJobs(pool),
		outbox:    postgres.NewOutbox(pool),
		readiness: postgres.NewReadiness(pool),
		migrate:   func(ctx context.Context, dir string) error { return db.Migrate(ctx, pool, dir) },
		close:     pool.Close,
	}, nil
}

func runWithDatabase(ctx context.Context, cfg config, logger *slog.Logger,
	open func(context.Context, string) (*databaseHandle, error),
	start func(context.Context, config, *slog.Logger, ports.HealthChecker, ports.AlertRepository, ports.JobRepository, ports.OutboxRepository) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.databaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if strings.TrimSpace(cfg.migrationsDir) == "" {
		return errors.New("migration directory is required")
	}
	database, err := open(ctx, cfg.databaseURL)
	if err != nil {
		return errors.New("initialize database failed")
	}
	defer database.close()
	if err := database.migrate(ctx, cfg.migrationsDir); err != nil {
		return errors.New("apply database migrations failed")
	}
	return start(ctx, cfg, logger, app.NewHealthWithReadiness(database.readiness), database.alerts, database.jobs, database.outbox)
}

func runServers(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker, alerts ...*app.Alerts) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	grpcListener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.grpcPort)))
	if err != nil {
		return errors.New("bind gRPC listener failed; check GRPC_PORT availability")
	}
	defer grpcListener.Close()
	httpListener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.httpPort)))
	if err != nil {
		return errors.New("bind HTTP listener failed; check HTTP_PORT availability")
	}
	defer httpListener.Close()
	return serve(ctx, logger, checker, grpcListener, httpListener, alerts...)
}

// serve owns both listeners and stops the other server when either one fails.
func serve(ctx context.Context, logger *slog.Logger, checker ports.HealthChecker, grpcListener, httpListener net.Listener, alerts ...*app.Alerts) error {
	defer grpcListener.Close()
	defer httpListener.Close()
	grpcServer := grpc.NewServer()
	if len(alerts) > 0 {
		jobalertv1.RegisterJobAlertServiceServer(grpcServer, grpcadapter.NewAlertHandler(alerts[0]))
	}
	healthpb.RegisterHealthServer(grpcServer, grpcadapter.NewHealthHandler(checker))
	httpServer := &http.Server{Handler: healthHandler(checker), ReadHeaderTimeout: 5 * time.Second}
	results := make(chan error, 2)
	go func() {
		_ = grpcServer.Serve(grpcListener)
		results <- errors.New("gRPC server stopped unexpectedly")
	}()
	go func() {
		_ = httpServer.Serve(httpListener)
		results <- errors.New("HTTP server stopped unexpectedly")
	}()
	logger.Info("servers starting", "grpc_address", grpcListener.Addr().String(), "http_address", httpListener.Addr().String())
	var serveErr error
	remaining := 2
	select {
	case <-ctx.Done():
	case serveErr = <-results:
		remaining--
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := shutdown(shutdownCtx, grpcServer, httpServer)
	for range remaining {
		<-results
	}
	logger.Info("servers stopped")
	return errors.Join(serveErr, shutdownErr)
}

func shutdown(ctx context.Context, grpcServer *grpc.Server, httpServer *http.Server) error {
	grpcDone := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcDone) }()
	var shutdownErr error
	if err := httpServer.Shutdown(ctx); err != nil {
		_ = httpServer.Close()
		shutdownErr = errors.New("HTTP graceful shutdown timed out or failed")
	}
	select {
	case <-grpcDone:
	case <-ctx.Done():
		grpcServer.Stop()
		<-grpcDone
		shutdownErr = errors.Join(shutdownErr, errors.New("gRPC graceful shutdown timed out"))
	}
	return shutdownErr
}
