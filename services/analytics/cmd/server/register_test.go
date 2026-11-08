package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

// stubOutcomes is a fixed repository for wiring tests; the real one is
// covered by the Testcontainers integration test.
type stubOutcomes struct{}

func (stubOutcomes) RecordEvent(context.Context, domain.OutcomeEvent) (bool, error) {
	return false, nil
}

func (stubOutcomes) CountEvents(context.Context, domain.TimeRange) ([]domain.EventCount, error) {
	return []domain.EventCount{{EventType: domain.EventJobPublished, Count: 3}}, nil
}

// recordingRegistrar keeps what registerAnalytics registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterAnalytics_RegistersPublicAndInternalServices(t *testing.T) {
	var reg recordingRegistrar
	registerAnalytics(stubOutcomes{})(&reg)
	if len(reg.descs) != 2 ||
		reg.descs[0].ServiceName != "jobtrack.analytics.v1.AnalyticsService" ||
		reg.descs[1].ServiceName != "jobtrack.analytics.v1.AnalyticsInternalService" {
		t.Fatalf("registered %v, want AnalyticsService and AnalyticsInternalService", reg.descs)
	}
	if _, ok := reg.impls[0].(analyticsv1.AnalyticsServiceServer); !ok {
		t.Fatalf("registered %T, not an AnalyticsServiceServer", reg.impls[0])
	}
	if _, ok := reg.impls[1].(analyticsv1.AnalyticsInternalServiceServer); !ok {
		t.Fatalf("registered %T, not an AnalyticsInternalServiceServer", reg.impls[1])
	}
}

func TestInternalServerOptions_RejectsInvalidTokens(t *testing.T) {
	for _, token := range []string{"", "private-secret value", "private-secret\u00e9"} {
		opts, err := internalServerOptions(token)
		if err == nil || opts != nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("token %q: opts=%v err=%v", token, opts, err)
		}
	}
	if opts, err := internalServerOptions("valid-token"); err != nil || len(opts) != 1 {
		t.Fatalf("valid token: opts=%v err=%v", opts, err)
	}
}

// End to end over a real gRPC server with the real JT-44 interceptor, as
// run wires it: only weekly-summary with the right token reaches the
// internal RPC, and the public RPC is unchanged.
func TestServe_InternalInterceptorGuardsOnlyTheInternalService(t *testing.T) {
	const token = "test-internal-token"
	// The same options run passes to the server.
	serverOpts, err := internalServerOptions(token)
	if err != nil {
		t.Fatal(err)
	}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerAnalytics(stubOutcomes{}),
			grpcListener, httpListener, serverOpts...)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})

	dial := func(t *testing.T, interceptor grpc.UnaryClientInterceptor) *grpc.ClientConn {
		t.Helper()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if interceptor != nil {
			opts = append(opts, grpc.WithChainUnaryInterceptor(interceptor))
		}
		conn, err := grpc.NewClient(grpcListener.Addr().String(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	internalClient := func(t *testing.T, token, caller string) *grpc.ClientConn {
		t.Helper()
		interceptor, err := grpcclient.InternalUnaryClientInterceptor(token, caller)
		if err != nil {
			t.Fatal(err)
		}
		return dial(t, interceptor)
	}
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	internalReq := &analyticsv1.GetInternalSummaryRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(7 * 24 * time.Hour))}
	publicReq := &analyticsv1.GetSummaryRequest{StartTime: internalReq.StartTime, EndTime: internalReq.EndTime}
	rpcCtx := func(t *testing.T) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return c
	}

	for _, tt := range []struct {
		name string
		conn func(t *testing.T) *grpc.ClientConn
		want codes.Code
	}{
		{"weekly-summary with the right token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, token, "weekly-summary") }, codes.OK},
		{"wrong token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, "wrong-token", "weekly-summary") }, codes.Unauthenticated},
		{"another internal caller", func(t *testing.T) *grpc.ClientConn {
			return internalClient(t, token, "notification-dispatcher")
		}, codes.PermissionDenied},
		{"public client without a token", func(t *testing.T) *grpc.ClientConn { return dial(t, grpcclient.UnaryClientInterceptor()) }, codes.Unauthenticated},
	} {
		t.Run("GetInternalSummary: "+tt.name, func(t *testing.T) {
			resp, err := analyticsv1.NewAnalyticsInternalServiceClient(tt.conn(t)).GetInternalSummary(rpcCtx(t), internalReq)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && resp.GetSummary().GetJobsPublished() != 3 {
				t.Fatalf("summary %v, want jobs_published 3", resp.GetSummary())
			}
		})
	}

	// The public RPC needs no internal token. These headers stand in for the
	// identity the gateway forwards.
	for _, tt := range []struct {
		role string
		want codes.Code
	}{
		{"admin", codes.OK},
		{"employer", codes.PermissionDenied},
	} {
		t.Run("GetSummary as "+tt.role, func(t *testing.T) {
			c := metadata.AppendToOutgoingContext(rpcCtx(t), "x-user-id", "22222222-2222-4222-8222-222222222222", "x-user-role", tt.role)
			resp, err := analyticsv1.NewAnalyticsServiceClient(dial(t, nil)).GetSummary(c, publicReq)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && resp.GetSummary().GetJobsPublished() != 3 {
				t.Fatalf("summary %v, want jobs_published 3", resp.GetSummary())
			}
		})
	}
}
