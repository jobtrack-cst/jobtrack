package grpc_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	adapter "github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type healthCheckerFunc func(context.Context) (domain.HealthStatus, error)

func (f healthCheckerFunc) CheckHealth(ctx context.Context) (domain.HealthStatus, error) {
	return f(ctx)
}

func TestHealthHandler_Check_DelegatesAndMapsResults(t *testing.T) {
	for _, tt := range []struct {
		name   string
		health domain.HealthStatus
		err    error
		want   healthpb.HealthCheckResponse_ServingStatus
		code   codes.Code
	}{
		{"healthy", domain.Serving, nil, healthpb.HealthCheckResponse_SERVING, codes.OK},
		{"unhealthy", domain.NotServing, nil, healthpb.HealthCheckResponse_NOT_SERVING, codes.OK},
		{"unknown", domain.Unknown, nil, healthpb.HealthCheckResponse_UNKNOWN, codes.OK},
		{"invalid domain status", domain.HealthStatus(255), nil, 0, codes.Internal},
		{"application failure", domain.Unknown, errors.New("private database password"), 0, codes.Internal},
		{"application status error", domain.Unknown, status.Error(codes.PermissionDenied, "private database password"), 0, codes.Internal},
		{"application cancellation", domain.Unknown, context.Canceled, 0, codes.Canceled},
		{"application deadline", domain.Unknown, context.DeadlineExceeded, 0, codes.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, "caller")
			calls := 0
			handler := adapter.NewHealthHandler(healthCheckerFunc(func(got context.Context) (domain.HealthStatus, error) {
				calls++
				if got != ctx {
					t.Error("caller context was not preserved")
				}
				return tt.health, tt.err
			}))
			response, err := handler.Check(ctx, &healthpb.HealthCheckRequest{})
			if calls != 1 || status.Code(err) != tt.code {
				t.Fatalf("calls=%d err=%v, want code %v", calls, err, tt.code)
			}
			if tt.code == codes.OK {
				if response == nil || response.Status != tt.want {
					t.Fatalf("got %v, want %v", response, tt.want)
				}
			} else if response != nil {
				t.Error("error returned a response")
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("application details leaked")
			}
		})
	}
}

func TestHealthHandler_Check_RejectsInvalidRequestsBeforeApplication(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		req  *healthpb.HealthCheckRequest
		code codes.Code
	}{
		{"unknown service", context.Background(), &healthpb.HealthCheckRequest{Service: "unknown.Service"}, codes.NotFound},
		{"nil request", context.Background(), nil, codes.InvalidArgument},
		{"canceled", canceled, &healthpb.HealthCheckRequest{}, codes.Canceled},
		{"expired", expired, &healthpb.HealthCheckRequest{}, codes.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := adapter.NewHealthHandler(healthCheckerFunc(func(context.Context) (domain.HealthStatus, error) {
				t.Error("invalid request reached application")
				return domain.Serving, nil
			}))
			response, err := handler.Check(tt.ctx, tt.req)
			if response != nil || status.Code(err) != tt.code {
				t.Fatalf("response=%v err=%v; want %v", response, err, tt.code)
			}
		})
	}
}

func TestHealthHandler_Check_ThroughGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, adapter.NewHealthHandler(app.NewHealth()))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
		if err := <-served; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve: %v", err)
		}
	})
	conn, err := grpc.NewClient("passthrough:///health-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := healthpb.NewHealthClient(conn)
	for _, tt := range []struct {
		name string
		code codes.Code
	}{
		{"", codes.OK}, {"unknown.Service", codes.NotFound},
	} {
		t.Run("service="+tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			response, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: tt.name})
			if status.Code(err) != tt.code {
				t.Fatalf("got %v, want code %v", err, tt.code)
			}
			if tt.code == codes.OK && (response == nil || response.Status != healthpb.HealthCheckResponse_SERVING) {
				t.Fatalf("got %v, want SERVING", response)
			}
		})
	}
}
