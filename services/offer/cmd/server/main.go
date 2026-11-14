package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	grpcadapter "github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	serviceName   string
	grpcPort      int
	httpPort      int
	databaseURL   string
	migrationsDir string
	rabbitMQURL   string
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logging.New("offer", os.Stdout).Error("invalid startup configuration", "error", err)
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
	cfg.migrationsDir = "services/offer/migrations"
	cfg.rabbitMQURL = getenv("RABBITMQ_URL")
	if strings.TrimSpace(cfg.rabbitMQURL) == "" {
		return config{}, errors.New("RABBITMQ_URL is required")
	}
	return cfg, nil
}

// deps are the resources available once the database is migrated.
type deps struct {
	checker      ports.HealthChecker
	offers       ports.OfferRepository
	participants ports.ParticipantRepository
	outbox       ports.OutboxRepository
	recorder     rabbitmq.ApplicationRecorder
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	return runWithDatabase(ctx, cfg, logger, openDatabase,
		func(ctx context.Context, cfg config, logger *slog.Logger, d deps) error {
			return runWithPublisher(ctx, cfg, logger, d,
				func(url string) (publisherResource, error) { return rabbitmq.NewPublisher(url) },
				func(ctx context.Context, cfg config, logger *slog.Logger, d deps) error {
					return runServers(ctx, cfg, logger, d.checker, registerOffer(d.offers, d.participants))
				})
		})
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// registerOffer registers public operations backed by the local projection.
func registerOffer(offers ports.OfferRepository, participants ports.ParticipantRepository) func(grpc.ServiceRegistrar) {
	handler := grpcadapter.NewOfferHandler(app.NewOffers(offers, participants, systemClock{}))
	return func(s grpc.ServiceRegistrar) { offerv1.RegisterOfferServiceServer(s, handler) }
}

type publisherResource interface {
	ports.EventPublisher
	Close() error
}

func runWithPublisher(ctx context.Context, cfg config, logger *slog.Logger, d deps,
	open func(string) (publisherResource, error),
	start func(context.Context, config, *slog.Logger, deps) error,
) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.rabbitMQURL) == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	publisher, err := open(cfg.rabbitMQURL)
	if err != nil {
		return errors.New("initialize RabbitMQ publisher failed")
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			err = errors.Join(err, errors.New("close RabbitMQ publisher failed"))
		}
	}()
	if d.outbox == nil || d.recorder == nil {
		return start(ctx, cfg, logger, d)
	}
	recovering := newRecoveringPublisher(publisher, func() (publisherResource, error) { return open(cfg.rabbitMQURL) })
	defer recovering.Close()
	workerCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		if err := events.Consume(workerCtx, rabbitmq.ApplicationConsumerConfig(cfg.rabbitMQURL, logger, d.recorder)); err != nil && workerCtx.Err() == nil {
			logger.Error("application consumer stopped", "error", err)
		}
	}()
	defer func() { cancel(); workers.Wait() }()
	relay := app.NewOutboxRelay(d.outbox, recovering, systemClock{})
	return runWithRelay(workerCtx, logger, relay, outboxRelayInterval, func(ctx context.Context) error { return start(ctx, cfg, logger, d) })
}

// databaseHandle holds operations on one owned pool, without creating another pool manager.
type databaseHandle struct {
	readiness    ports.Readiness
	offers       ports.OfferRepository
	participants ports.ParticipantRepository
	migrate      func(context.Context, string) error
	outbox       ports.OutboxRepository
	recorder     rabbitmq.ApplicationRecorder
	close        func()
}

func openDatabase(ctx context.Context, url string) (*databaseHandle, error) {
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		return nil, err
	}
	return &databaseHandle{
		readiness:    postgres.NewReadiness(pool),
		outbox:       postgres.NewOutbox(pool),
		recorder:     postgres.NewParticipants(pool),
		offers:       postgres.NewOffers(pool),
		participants: postgres.NewParticipants(pool),
		migrate:      func(ctx context.Context, dir string) error { return db.Migrate(ctx, pool, dir) },
		close:        pool.Close,
	}, nil
}

func runWithDatabase(ctx context.Context, cfg config, logger *slog.Logger,
	open func(context.Context, string) (*databaseHandle, error),
	start func(context.Context, config, *slog.Logger, deps) error,
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
	return start(ctx, cfg, logger, deps{checker: app.NewHealthWithReadiness(database.readiness), offers: database.offers, participants: database.participants, outbox: database.outbox, recorder: database.recorder})
}

func runServers(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker, register func(grpc.ServiceRegistrar)) error {
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
	return serve(ctx, logger, checker, register, grpcListener, httpListener)
}

// serve owns both listeners and stops the other server when either one fails.
func serve(ctx context.Context, logger *slog.Logger, checker ports.HealthChecker, register func(grpc.ServiceRegistrar), grpcListener, httpListener net.Listener) error {
	defer grpcListener.Close()
	defer httpListener.Close()
	grpcServer := grpc.NewServer()
	healthpb.RegisterHealthServer(grpcServer, grpcadapter.NewHealthHandler(checker))
	if register != nil {
		register(grpcServer)
	}
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
