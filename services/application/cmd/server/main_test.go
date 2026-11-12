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
	applicationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type checkerFunc func(context.Context) (domain.HealthStatus, error)

func (f checkerFunc) CheckHealth(ctx context.Context) (domain.HealthStatus, error) { return f(ctx) }

func TestHealthHandler_HTTPStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		health domain.HealthStatus
		err    error
		code   int
	}{
		{"healthy", domain.Serving, nil, http.StatusOK},
		{"not serving", domain.NotServing, nil, http.StatusServiceUnavailable},
		{"unknown", domain.Unknown, nil, http.StatusServiceUnavailable},
		{"application failure", domain.Unknown, errors.New("private credentials"), http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			calls := 0
			handler := healthHandler(checkerFunc(func(ctx context.Context) (domain.HealthStatus, error) {
				calls++
				if ctx != req.Context() {
					t.Error("request context changed")
				}
				return tt.health, tt.err
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if calls != 1 || response.Code != tt.code {
				t.Fatalf("calls=%d status=%d", calls, response.Code)
			}
			if strings.Contains(response.Body.String(), "private") {
				t.Fatal("error details leaked")
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	for _, tt := range []struct {
		name     string
		env      map[string]string
		want     config
		errField string
	}{
		{"defaults", map[string]string{"SERVICE_NAME": "application"}, config{serviceName: "application", grpcPort: 9090, httpPort: 8080}, ""},
		{"custom", map[string]string{"SERVICE_NAME": "job-posting", "GRPC_PORT": "19090", "HTTP_PORT": "18080"}, config{serviceName: "job-posting", grpcPort: 19090, httpPort: 18080}, ""},
		{"boundary ports", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "1", "HTTP_PORT": "65535"}, config{serviceName: "test", grpcPort: 1, httpPort: 65535}, ""},
		{"missing job address", map[string]string{"SERVICE_NAME": "application", "JOB_POSTING_ADDR": ""}, config{}, "JOB_POSTING_ADDR"},
		{"blank resume address", map[string]string{"SERVICE_NAME": "application", "RESUME_ADDR": " \t"}, config{}, "RESUME_ADDR"},
		{"missing broker URL", map[string]string{"SERVICE_NAME": "application", "RABBITMQ_URL": ""}, config{}, "RABBITMQ_URL"},
		{"missing name", nil, config{}, "SERVICE_NAME"},
		{"invalid name", map[string]string{"SERVICE_NAME": "Invalid Name"}, config{}, "SERVICE_NAME"},
		{"invalid gRPC port", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "private-secret"}, config{}, "GRPC_PORT"},
		{"zero port", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "0"}, config{}, "GRPC_PORT"},
		{"negative port", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "-1"}, config{}, "HTTP_PORT"},
		{"overflow port", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "65536"}, config{}, "HTTP_PORT"},
		{"same ports", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "9090"}, config{}, "must differ"},
		{"missing database URL", map[string]string{"SERVICE_NAME": "test", "DATABASE_URL": ""}, config{}, "DATABASE_URL"},
		{"blank database URL", map[string]string{"SERVICE_NAME": "test", "DATABASE_URL": " \t"}, config{}, "DATABASE_URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(func(key string) string {
				if key == "DATABASE_URL" || key == "RABBITMQ_URL" || key == "JOB_POSTING_ADDR" || key == "RESUME_ADDR" {
					if value, ok := tt.env[key]; ok {
						return value
					}
					return "caller-supplied-url"
				}
				return tt.env[key]
			})
			if tt.errField == "" {
				tt.want.databaseURL = "caller-supplied-url"
				tt.want.rabbitMQURL = "caller-supplied-url"
				tt.want.jobPostingAddr = "caller-supplied-url"
				tt.want.resumeAddr = "caller-supplied-url"
				tt.want.migrationsDir = "services/application/migrations"
			}
			if tt.errField != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errField) || strings.Contains(err.Error(), "private-secret") {
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
			err := runWithDatabase(ctx, cfg, logging.New("test", io.Discard),
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
				}, func(got context.Context, _ config, _ *slog.Logger, checker ports.HealthChecker, _ ports.Repository, _ ports.OutboxRepository) error {
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
			case "missing URL", "missing directory":
				want = nil
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want %v", calls, want)
			}
			wantErr := stage != "success" && stage != "shutdown"
			if (err != nil) != wantErr {
				t.Fatalf("unexpected result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("database details leaked")
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
	t.Cleanup(func() { l.Close() })
	return l
}

func awaitServer(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(8 * time.Second):
		t.Fatal("server did not stop")
		return nil
	}
}

func assertListenerClosed(t *testing.T, address string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Errorf("listener %s remains open", address)
	}
}

func TestServe_RegistersHealthAndShutsDownBothServers(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("application", &logs), app.NewHealth(), grpcListener, httpListener, app.NewService(nil, nil, nil))
	}()
	t.Cleanup(func() { cancel() })
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + httpListener.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "ok\n" {
		t.Fatalf("HTTP response %d %q %v", response.StatusCode, body, err)
	}
	response, err = client.Get("http://" + httpListener.Addr().String() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !strings.HasSuffix(string(body), "jobtrack_service_ready 1\n") {
		t.Fatalf("metrics response %d %q %v", response.StatusCode, body, err)
	}
	if got := response.Header.Get("Content-Type"); got != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics Content-Type %q", got)
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
	// Exercise registration through the real gRPC server, not just the handler.
	_, rpcErr := applicationv1.NewApplicationServiceClient(conn).GetApplication(rpcCtx, &applicationv1.GetApplicationRequest{})
	if status.Code(rpcErr) != codes.Unauthenticated {
		t.Fatalf("application RPC was not registered: %v", rpcErr)
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
		if err := json.Unmarshal(line, &entry); err != nil || entry["service"] != "application" {
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
			grpcAddress, httpAddress := grpcListener.Addr().String(), httpListener.Addr().String()
			if fail == "gRPC" {
				grpcListener = failingListener{grpcListener}
			} else {
				httpListener = failingListener{httpListener}
			}
			done := make(chan error, 1)
			go func() {
				done <- serve(context.Background(), logging.New("test", io.Discard), app.NewHealth(), grpcListener, httpListener)
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
	// Match run's wildcard binding: loopback alone can leave IPv6 available.
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { occupied.Close() })
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	port := occupied.Addr().(*net.TCPAddr).Port
	logger := logging.New("test", io.Discard)
	if err := runServers(ctx, config{serviceName: "test", grpcPort: port, httpPort: 0}, logger, app.NewHealth()); err == nil || !strings.Contains(err.Error(), "gRPC") {
		t.Fatalf("expected gRPC binding error: %v", err)
	}
	available := listenForTest(t)
	grpcPort := available.Addr().(*net.TCPAddr).Port
	available.Close()
	if err := runServers(ctx, config{serviceName: "test", grpcPort: grpcPort, httpPort: port}, logger, app.NewHealth()); err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("expected HTTP binding error: %v", err)
	}
	assertListenerClosed(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(grpcPort)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServers(ctx, config{serviceName: "test", grpcPort: 0, httpPort: 0}, logger, app.NewHealth()); !errors.Is(err, context.Canceled) {
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

// pgx, not our configuration loader, resolves the separate password environment.
func TestLoadConfig_PgxPasswordEnvironmentPreservesReservedCharacters(t *testing.T) {
	for _, character := range []string{"@", ":", "/", "?", "#", "%", "@:/?#%", "'\\ space"} {
		t.Run(character, func(t *testing.T) {
			password := "synthetic-" + character + "-password"
			t.Setenv("PGPASSWORD", password)
			cfg, err := loadConfig(func(key string) string {
				return map[string]string{"SERVICE_NAME": "application", "JOB_POSTING_ADDR": "job-posting:9090", "RESUME_ADDR": "resume:9090", "RABBITMQ_URL": "amqp://localhost", "DATABASE_URL": "postgres://application@postgres:5432/application?sslmode=disable"}[key]
			})
			if err != nil {
				t.Fatal("load configuration failed")
			}
			parsed, err := pgxpool.ParseConfig(cfg.databaseURL)
			if err != nil {
				t.Fatal("pgx configuration parse failed")
			}
			if parsed.ConnConfig.Host != "postgres" || parsed.ConnConfig.Port != 5432 || parsed.ConnConfig.User != "application" || parsed.ConnConfig.Database != "application" || parsed.ConnConfig.Password != password {
				t.Fatal("pgx changed database endpoint or password")
			}
		})
	}
}

func TestRun_RejectsCanceledContextBeforeOpeningDatabase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(ctx, config{databaseURL: "postgres://invalid%zz", migrationsDir: "services/application/migrations"}, logging.New("application", io.Discard))
	if !errors.Is(err, context.Canceled) {
		t.Fatal("canceled startup did not stop before opening database")
	}
}

func TestOpenDatabase_InvalidConfigurationDoesNotLeakCredentials(t *testing.T) {
	database, err := openDatabase(context.Background(), "postgres://synthetic-user:synthetic-password@invalid%zz")
	if err == nil || database != nil {
		t.Fatal("invalid configuration was accepted")
	}
	if strings.Contains(err.Error(), "synthetic-password") {
		t.Fatal("database error exposed credentials")
	}
}
