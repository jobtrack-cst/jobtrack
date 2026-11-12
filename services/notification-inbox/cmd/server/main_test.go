package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type checkerFunc func(context.Context) (domain.HealthStatus, error)

func (f checkerFunc) CheckHealth(ctx context.Context) (domain.HealthStatus, error) { return f(ctx) }
func (f checkerFunc) Check(ctx context.Context) error {
	status, err := f(ctx)
	if err != nil {
		return err
	}
	if !status.IsHealthy() {
		return errors.New("not healthy")
	}
	return nil
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config
		wantErr string
	}{
		{
			name: "valid config",
			env: map[string]string{
				"SERVICE_NAME": "notification-inbox",
				"DATABASE_URL": "postgres://localhost",
				"GRPC_PORT":    "9090",
				"HTTP_PORT":    "8080",
			},
			want: config{
				serviceName:   "notification-inbox",
				databaseURL:   "postgres://localhost",
				migrationsDir: "services/notification-inbox/migrations",
				grpcPort:      9090,
				httpPort:      8080,
			},
		},
		{
			name: "missing DATABASE_URL",
			env: map[string]string{
				"SERVICE_NAME": "notification-inbox",
			},
			wantErr: "DATABASE_URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(func(k string) string { return tt.env[k] })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("unexpected error %v", err)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

type readinessFunc func(context.Context) error

func (f readinessFunc) Check(ctx context.Context) error { return f(ctx) }

func TestRunWithDatabase_OrdersStartupAndCleansUp(t *testing.T) {
	for _, stage := range []string{"success", "connection failure", "migration failure", "listener failure", "shutdown", "missing URL", "missing directory"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := config{databaseURL: "caller-supplied-url", migrationsDir: t.TempDir()}
			if stage == "missing URL" {
				cfg.databaseURL = " "
			}
			if stage == "missing directory" {
				cfg.migrationsDir = ""
			}
			var calls []string
			_ = runWithDatabase(ctx, cfg, logging.New("test", io.Discard),
				func(got context.Context, url string) (*databaseHandle, error) {
					calls = append(calls, "open")
					if got != ctx || url != cfg.databaseURL {
						t.Error("connection configuration changed")
					}
					if stage == "connection failure" {
						return nil, errors.New("private database credentials")
					}
					return &databaseHandle{
						readiness: readinessFunc(func(context.Context) error { calls = append(calls, "ready"); return nil }),
						migrate: func(got context.Context, dir string) error {
							calls = append(calls, "migrate")
							if got != ctx || dir != cfg.migrationsDir {
								t.Error("migration configuration changed")
							}
							if stage == "migration failure" {
								return errors.New("private SQL details")
							}
							return nil
						},
						close: func() { calls = append(calls, "close") },
					}, nil
				}, func(got context.Context, _ config, _ *slog.Logger, checker ports.HealthChecker, _ *pgxpool.Pool) error {
					calls = append(calls, "start")
					if stage == "listener failure" {
						return errors.New("bind failed")
					}
					health, err := checker.CheckHealth(got)
					if err != nil || !health.IsHealthy() {
						t.Fatalf("wired readiness failed: %v", err)
					}
					if stage == "shutdown" {
						cancel()
						<-got.Done()
					}
					return nil
				})
			want := []string{"open", "migrate", "start", "ready", "close"}
			switch stage {
			case "connection failure":
				want = []string{"open"}
			case "migration failure":
				want = []string{"open", "migrate", "close"}
			case "listener failure":
				want = []string{"open", "migrate", "start", "close"}
			case "missing URL":
				want = nil
			case "missing directory":
				want = nil
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want %v", calls, want)
			}
		})
	}
}

func listenForTest(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func assertListenerClosed(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatalf("listener still open at %s", addr)
	}
}

func awaitServer(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to stop")
		return nil
	}
}

func TestServe_RegistersHealthAndShutsDownBothServers(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("notification-inbox", &logs), app.NewHealth(), nil, grpcListener, httpListener)
	}()
	t.Cleanup(func() { cancel() })
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + httpListener.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", response.StatusCode)
	}
	conn, err := grpc.NewClient(grpcListener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rpcCancel()
	health, err := healthpb.NewHealthClient(conn).Check(rpcCtx, &healthpb.HealthCheckRequest{})
	if err != nil || health.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("gRPC response %v, %v", health, err)
	}
	cancel()
	if err := awaitServer(t, done); err != nil {
		t.Fatal(err)
	}
	assertListenerClosed(t, grpcListener.Addr().String())
	assertListenerClosed(t, httpListener.Addr().String())
	lines := bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected startup and shutdown logs: %s", logs.String())
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil || entry["service"] != "notification-inbox" {
			t.Fatalf("invalid service JSON log: %s", line)
		}
	}
}

type failingListener struct{ net.Listener }

func (f failingListener) Accept() (net.Conn, error) {
	return nil, errors.New("private listener failure")
}

func TestServe_ServerFailureClosesBothListeners(t *testing.T) {
	for _, fail := range []string{"gRPC", "HTTP"} {
		t.Run(fail, func(t *testing.T) {
			grpcListener, httpListener := listenForTest(t), listenForTest(t)
			grpcAddress := grpcListener.Addr().String()
			httpAddress := httpListener.Addr().String()
			if fail == "gRPC" {
				grpcListener = failingListener{grpcListener}
			} else {
				httpListener = failingListener{httpListener}
			}
			done := make(chan error, 1)
			go func() {
				done <- serve(context.Background(), logging.New("test", io.Discard), app.NewHealth(), nil, grpcListener, httpListener)
			}()
			err := awaitServer(t, done)
			if err == nil || !strings.Contains(err.Error(), fail) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected error %v", err)
			}
			assertListenerClosed(t, grpcAddress)
			assertListenerClosed(t, httpAddress)
		})
	}
}

func TestRun_BindingErrorsCleanUpListeners(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { occupied.Close() })
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	port := occupied.Addr().(*net.TCPAddr).Port
	logger := logging.New("test", io.Discard)
	if err := runServers(ctx, config{serviceName: "test", grpcPort: port, httpPort: 0}, logger, app.NewHealth(), nil); err == nil || !strings.Contains(err.Error(), "gRPC") {
		t.Fatalf("expected gRPC binding error: %v", err)
	}
	available := listenForTest(t)
	grpcPort := available.Addr().(*net.TCPAddr).Port
	available.Close()
	if err := runServers(ctx, config{serviceName: "test", grpcPort: grpcPort, httpPort: port}, logger, app.NewHealth(), nil); err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("expected HTTP binding error: %v", err)
	}
	assertListenerClosed(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(grpcPort)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServers(ctx, config{serviceName: "test", grpcPort: 0, httpPort: 0}, logger, app.NewHealth(), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}

func TestShutdown_DeadlineClosesActiveHTTPConnections(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		response, err := server.Client().Get(server.URL)
		if err == nil {
			response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP request did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := shutdown(ctx, grpc.NewServer(), server.Config); err == nil {
		t.Fatal("expected graceful shutdown deadline error")
	}
	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("forced shutdown left request running")
	}
}
