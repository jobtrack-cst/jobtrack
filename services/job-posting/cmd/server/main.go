package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/pkg/requestmetrics"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/clients"
	grpcadapter "github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	serviceName   string
	grpcPort      int
	httpPort      int
	databaseURL   string
	migrationsDir string
	rabbitMQURL   string
	internalToken string
	employerAddr  string
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logging.New("job-posting", os.Stdout).Error("invalid startup configuration", "error", err)
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
	cfg.migrationsDir = "services/job-posting/migrations"
	cfg.rabbitMQURL = getenv("RABBITMQ_URL")
	if strings.TrimSpace(cfg.rabbitMQURL) == "" {
		return config{}, errors.New("RABBITMQ_URL is required")
	}
	cfg.employerAddr = getenv("EMPLOYER_ADDR")
	if cfg.employerAddr == "" {
		return config{}, errors.New("EMPLOYER_ADDR is required")
	}
	cfg.internalToken = getenv("INTERNAL_TOKEN")
	if !authctx.ValidInternalToken(cfg.internalToken) {
		return config{}, errors.New("INTERNAL_TOKEN must contain nonempty visible ASCII without whitespace")
	}
	return cfg, nil
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

type deps struct {
	checker ports.HealthChecker
	jobs    ports.JobRepository
	outbox  ports.OutboxRepository
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	serverOpts, err := internalServerOptions(cfg.internalToken)
	if err != nil {
		return err
	}
	verifier, closeEmployer, err := newEmployerVerifier(cfg.employerAddr)
	if err != nil {
		return err
	}
	defer closeEmployer()

	return runWithDatabase(ctx, cfg, logger, openDatabase,
		func(ctx context.Context, cfg config, logger *slog.Logger, d deps) error {
			// 1. Start the outbox relay
			pub, err := rabbitmq.NewPublisher(cfg.rabbitMQURL)
			if err != nil {
				return errors.New("initialize RabbitMQ publisher failed")
			}
			defer pub.Close()

			relay := app.NewOutboxRelay(d.outbox, pub, wallClock{})

			return runWithRelay(ctx, logger, relay, outboxRelayInterval, func(ctx context.Context) error {
				// 2. Start the moderation consumer
				consumerCtx, stopConsumer := context.WithCancel(ctx)
				defer stopConsumer()

				go func() {
					_ = events.Consume(consumerCtx, rabbitmq.ModerationConsumerConfig(cfg.rabbitMQURL, logger, d.jobs))
				}()

				// 3. Run the gRPC servers
				return runServers(ctx, cfg, logger, d.checker, registerJobPosting(d.jobs, verifier), serverOpts...)
			})
		})
}

func internalServerOptions(token string) ([]grpc.ServerOption, error) {
	internalAuth, err := authctx.InternalUnaryServerInterceptor(token)
	if err != nil {
		return nil, errors.New("INTERNAL_TOKEN is invalid")
	}
	return []grpc.ServerOption{grpc.ChainUnaryInterceptor(internalAuth)}, nil
}

func newEmployerVerifier(addr string) (ports.EmployerVerifier, func(), error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()),
	)
	if err != nil {
		return nil, nil, errors.New("initialize employer client failed")
	}
	return clients.NewEmployerClient(employerv1.NewEmployerServiceClient(conn)), func() { conn.Close() }, nil
}

type databaseHandle struct {
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
	repo := postgres.NewJobs(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)
	return &databaseHandle{
		jobs:      repo,
		outbox:    outboxRepo,
		readiness: postgres.NewReadiness(pool),
		migrate:   func(ctx context.Context, dir string) error { return db.Migrate(ctx, pool, dir) },
		close:     pool.Close,
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
	return start(ctx, cfg, logger, deps{
		checker: app.NewHealthWithReadiness(database.readiness),
		jobs:    database.jobs,
		outbox:  database.outbox,
	})
}

func runServers(ctx context.Context, cfg config, logger *slog.Logger, checker ports.HealthChecker, register func(*grpc.Server), opts ...grpc.ServerOption) error {
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
	return serve(ctx, logger, checker, register, opts, grpcListener, httpListener)
}

func serve(ctx context.Context, logger *slog.Logger, checker ports.HealthChecker, register func(*grpc.Server), opts []grpc.ServerOption, grpcListener, httpListener net.Listener) error {
	defer grpcListener.Close()
	defer httpListener.Close()

	requestMetrics := requestmetrics.New("job-posting")
	opts = append([]grpc.ServerOption{grpc.ChainUnaryInterceptor(requestMetrics.UnaryServerInterceptor())}, opts...)
	grpcServer := grpc.NewServer(opts...)
	healthpb.RegisterHealthServer(grpcServer, grpcadapter.NewHealthHandler(checker))
	register(grpcServer)

	requestMetrics.RegisterGRPC(grpcServer)
	httpServer := &http.Server{Handler: healthHandler(checker, requestMetrics), ReadHeaderTimeout: 5 * time.Second}
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
	logger.Info("servers stopped")
	for range remaining {
		<-results
	}
	if serveErr != nil {
		return serveErr
	}
	return shutdownErr
}

func shutdown(ctx context.Context, grpcServer *grpc.Server, httpServer *http.Server) error {
	var errs []error
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		grpcServer.Stop()
		errs = append(errs, ctx.Err())
	}
	if err := httpServer.Shutdown(ctx); err != nil {
		_ = httpServer.Close()
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func registerJobPosting(repo ports.JobRepository, verifier ports.EmployerVerifier) func(*grpc.Server) {
	return func(s *grpc.Server) {
		svc := app.NewJobs(repo, verifier, wallClock{})
		public := grpcadapter.NewJobPostingHandler(svc)
		internal := grpcadapter.NewJobPostingInternalHandler(svc)
		jobpostingv1.RegisterJobPostingServiceServer(s, public)
		jobpostingv1.RegisterJobPostingInternalServiceServer(s, internal)
	}
}
