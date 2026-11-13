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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/backends"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/cors"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/middleware"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
)

var kebabCase = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

type config struct {
	serviceName string
	httpPort    int
	jwtSecret   string
	corsPolicy  cors.Policy       // zero value: CORS off
	addrs       map[string]string // backend name -> gRPC address
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logging.New("gateway", os.Stdout).Error("invalid startup configuration", "error", err)
		os.Exit(1)
	}
	logger := logging.New(cfg.serviceName, os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{serviceName: getenv("SERVICE_NAME"), addrs: map[string]string{}}
	if !kebabCase.MatchString(cfg.serviceName) {
		return config{}, errors.New("SERVICE_NAME must be a nonempty kebab-case name")
	}
	// The gateway must never hold the internal token (AGENTS.md Section 9).
	if getenv("INTERNAL_TOKEN") != "" {
		return config{}, errors.New("INTERNAL_TOKEN must not be set for the gateway")
	}
	cfg.httpPort = 8080
	if value := getenv("HTTP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return config{}, errors.New("HTTP_PORT must be an integer from 1 to 65535")
		}
		cfg.httpPort = port
	}
	cfg.jwtSecret = getenv("JWT_SECRET")
	if len(cfg.jwtSecret) < jwtauth.MinSecretBytes {
		return config{}, jwtauth.ErrWeakSecret
	}
	policy, err := cors.Parse(getenv("CORS_ALLOWED_ORIGIN"))
	if err != nil {
		return config{}, err
	}
	cfg.corsPolicy = policy
	var missing []string
	for _, b := range backends.All() {
		addr := strings.TrimSpace(getenv(b.AddrEnv()))
		if addr == "" {
			missing = append(missing, b.AddrEnv())
			continue
		}
		cfg.addrs[b.Name] = addr
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing backend addresses: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	verifier, err := jwtauth.NewVerifier(cfg.jwtSecret, nil)
	if err != nil {
		return err
	}
	api, closeConns, err := newAPI(ctx, cfg.addrs)
	if err != nil {
		return err
	}
	defer closeConns()
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(cfg.httpPort)))
	if err != nil {
		return errors.New("bind HTTP listener failed; check HTTP_PORT availability")
	}
	logger.Info("CORS configuration", "allowed_origin", cfg.corsPolicy.Origin())
	// CORS sits outside authentication: it answers preflights itself, and every other
	// request still goes through the JT-31 JWT check and header stripping.
	return serve(ctx, logger, cfg.corsPolicy.Wrap(newHandler(api, verifier, logger)), listener)
}

// newAPI registers every public backend through pkg/grpcclient. grpc.NewClient
// connects lazily, so the gateway starts even if some services are not running yet.
func newAPI(ctx context.Context, addrs map[string]string) (*runtime.ServeMux, func(), error) {
	mux := middleware.NewServeMux()
	var conns []*grpc.ClientConn
	closeAll := func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}
	for _, b := range backends.All() {
		conn, err := grpc.NewClient(addrs[b.Name],
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			// One interceptor per connection, so each backend has its own circuit breaker.
			grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()),
		)
		if err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("create client for %s: %w", b.Name, err)
		}
		conns = append(conns, conn)
		if err := b.Register(ctx, mux, conn); err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("register %s: %w", b.Name, err)
		}
	}
	return mux, closeAll, nil
}

func serve(ctx context.Context, logger *slog.Logger, handler http.Handler, listener net.Listener) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info("gateway starting", "http_address", listener.Addr().String(), "backends", len(backends.All()))
	select {
	case <-ctx.Done():
	case err := <-result:
		return fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	<-result
	logger.Info("gateway stopped")
	if shutdownErr != nil {
		return errors.New("HTTP graceful shutdown timed out or failed")
	}
	return nil
}
