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

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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
		{"defaults", map[string]string{"SERVICE_NAME": "moderation"}, config{serviceName: "moderation", grpcPort: 9090, httpPort: 8080}, ""},
		{"custom", map[string]string{"SERVICE_NAME": "job-posting", "GRPC_PORT": "19090", "HTTP_PORT": "18080"}, config{serviceName: "job-posting", grpcPort: 19090, httpPort: 18080}, ""},
		{"boundary ports", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "1", "HTTP_PORT": "65535"}, config{serviceName: "test", grpcPort: 1, httpPort: 65535}, ""},
		{"missing name", nil, config{}, "SERVICE_NAME"},
		{"invalid name", map[string]string{"SERVICE_NAME": "Invalid Name"}, config{}, "SERVICE_NAME"},
		{"invalid gRPC port", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "private-secret"}, config{}, "GRPC_PORT"},
		{"zero port", map[string]string{"SERVICE_NAME": "test", "GRPC_PORT": "0"}, config{}, "GRPC_PORT"},
		{"negative port", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "-1"}, config{}, "HTTP_PORT"},
		{"overflow port", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "65536"}, config{}, "HTTP_PORT"},
		{"same ports", map[string]string{"SERVICE_NAME": "test", "HTTP_PORT": "9090"}, config{}, "must differ"},
		{"missing database URL", map[string]string{"SERVICE_NAME": "test", "DATABASE_URL": ""}, config{}, "DATABASE_URL"},
		{"blank database URL", map[string]string{"SERVICE_NAME": "test", "DATABASE_URL": " \t"}, config{}, "DATABASE_URL"},
		{"missing RabbitMQ URL", map[string]string{"SERVICE_NAME": "test", "RABBITMQ_URL": ""}, config{}, "RABBITMQ_URL"},
		{"blank RabbitMQ URL", map[string]string{"SERVICE_NAME": "test", "RABBITMQ_URL": " \t"}, config{}, "RABBITMQ_URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(func(key string) string {
				if key == "DATABASE_URL" || key == "RABBITMQ_URL" {
					if value, ok := tt.env[key]; ok {
						return value
					}
					return "caller-supplied-url"
				}
				return tt.env[key]
			})
			if tt.errField == "" {
				tt.want.databaseURL = "caller-supplied-url"
				tt.want.migrationsDir = "services/moderation/migrations"
				tt.want.rabbitMQURL = "caller-supplied-url"
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

type startupPublisher struct {
	calls    *[]string
	closeErr error
}

func (p *startupPublisher) Publish(context.Context, *eventsv1.EventEnvelope) error {
	*p.calls = append(*p.calls, "unexpected publish")
	return nil
}
func (p *startupPublisher) Close() error {
	*p.calls = append(*p.calls, "close RabbitMQ")
	return p.closeErr
}

func TestStartup_RabbitMQOrderFailuresAndCleanup(t *testing.T) {
	for _, stage := range []string{"shutdown", "connection failure", "migration failure", "RabbitMQ failure", "listener failure", "close failure", "missing RabbitMQ URL"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := config{databaseURL: "database-config", migrationsDir: t.TempDir(), rabbitMQURL: "broker-config"}
			if stage == "missing RabbitMQ URL" {
				cfg.rabbitMQURL = " "
			}
			var calls []string
			err := runWithDatabase(ctx, cfg, logging.New("test", io.Discard),
				func(context.Context, string) (*databaseHandle, error) {
					calls = append(calls, "open database")
					if stage == "connection failure" {
						return nil, errors.New("private connection error")
					}
					return &databaseHandle{readiness: readinessFunc(func(context.Context) error { return nil }),
						migrate: func(context.Context, string) error {
							calls = append(calls, "migrate")
							if stage == "migration failure" {
								return errors.New("private SQL")
							}
							return nil
						},
						close: func() { calls = append(calls, "close database") },
					}, nil
				}, func(ctx context.Context, cfg config, logger *slog.Logger, d deps) error {
					return runWithPublisher(ctx, cfg, logger, d, func(url string) (publisherResource, error) {
						calls = append(calls, "open RabbitMQ")
						if url != cfg.rabbitMQURL {
							t.Error("broker URL changed")
						}
						if stage == "RabbitMQ failure" {
							return nil, errors.New("private broker credentials")
						}
						p := &startupPublisher{calls: &calls}
						if stage == "close failure" {
							p.closeErr = errors.New("private close error")
						}
						return p, nil
					}, func(context.Context, config, *slog.Logger, deps) error {
						calls = append(calls, "start servers")
						if stage == "listener failure" {
							return errors.New("bind failed")
						}
						cancel()
						<-ctx.Done()
						return nil
					})
				})
			want := []string{"open database", "migrate", "open RabbitMQ", "start servers", "close RabbitMQ", "close database"}
			switch stage {
			case "connection failure":
				want = []string{"open database"}
			case "migration failure":
				want = []string{"open database", "migrate", "close database"}
			case "RabbitMQ failure":
				want = []string{"open database", "migrate", "open RabbitMQ", "close database"}
			case "missing RabbitMQ URL":
				want = []string{"open database", "migrate", "close database"}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want %v", calls, want)
			}
			if (err != nil) != (stage != "shutdown") {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("credentials leaked")
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
						reports:   stubReports{},
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
				}, func(got context.Context, _ config, _ *slog.Logger, d deps) error {
					calls = append(calls, "start")
					if stage == "listener failure" {
						return errors.New("bind failed")
					}
					if d.reports == nil {
						t.Fatal("report repository was not wired")
					}
					health, err := d.checker.CheckHealth(got)
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
		done <- serve(ctx, logging.New("moderation", &logs), app.NewHealth(), nil, grpcListener, httpListener)
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
		if err := json.Unmarshal(line, &entry); err != nil || entry["service"] != "moderation" {
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
