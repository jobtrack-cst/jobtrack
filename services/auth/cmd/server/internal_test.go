package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const testInternalToken = "test-internal-token"

// fakeInternalAuth answers internal RPCs without a database, so these tests
// check only the interceptor wiring.
type fakeInternalAuth struct {
	authv1.UnimplementedAuthInternalServiceServer
}

func (fakeInternalAuth) ListUsersByRole(context.Context, *authv1.ListUsersByRoleRequest) (*authv1.ListUsersByRoleResponse, error) {
	return &authv1.ListUsersByRoleResponse{Users: []*authv1.User{{Email: "coordinator@cst.edu.bt", Role: authv1.Role_ROLE_ADMIN}}}, nil
}

func (fakeInternalAuth) GetUser(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	return &authv1.GetUserResponse{}, nil
}

func TestInternalServerOptions_RejectsInvalidTokens(t *testing.T) {
	for _, token := range []string{"", "private-secret value", "private-secret\u00e9"} {
		opts, err := internalServerOptions(token)
		if err == nil || opts != nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("token %q: opts=%v err=%v", token, opts, err)
		}
	}
	if opts, err := internalServerOptions(testInternalToken); err != nil || len(opts) != 1 {
		t.Fatalf("valid token: opts=%v err=%v", opts, err)
	}
}

func TestRun_InvalidInternalTokenStopsBeforeStartup(t *testing.T) {
	cfg := config{databaseURL: "caller-supplied-url", migrationsDir: t.TempDir(), rabbitMQURL: "caller-supplied-url", internalToken: "private-secret value"}
	err := run(context.Background(), cfg, logging.New("test", io.Discard))
	if err == nil || !strings.Contains(err.Error(), "INTERNAL_TOKEN") || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("unexpected error %v", err)
	}
}

// Over a real gRPC server with the JT-44 interceptor, as run wires it:
// ListUsersByRole is only for weekly-summary, GetUser only for
// notification-dispatcher, and the public AuthService is not guarded.
func TestServe_InternalInterceptorGuardsAuthInternalService(t *testing.T) {
	serverOpts, err := internalServerOptions(testInternalToken)
	if err != nil {
		t.Fatal(err)
	}
	register := func(s grpc.ServiceRegistrar) {
		authv1.RegisterAuthServiceServer(s, authv1.UnimplementedAuthServiceServer{})
		authv1.RegisterAuthInternalServiceServer(s, fakeInternalAuth{})
	}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), register, grpcListener, httpListener, serverOpts...)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})

	dial := func(t *testing.T, interceptor grpc.UnaryClientInterceptor) *grpc.ClientConn {
		t.Helper()
		conn, err := grpc.NewClient(grpcListener.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithChainUnaryInterceptor(interceptor))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	internal := func(t *testing.T, token, caller string) *grpc.ClientConn {
		t.Helper()
		interceptor, err := grpcclient.InternalUnaryClientInterceptor(token, caller)
		if err != nil {
			t.Fatal(err)
		}
		return dial(t, interceptor)
	}
	rpcCtx := func(t *testing.T) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return c
	}
	listReq := &authv1.ListUsersByRoleRequest{Role: authv1.Role_ROLE_ADMIN}

	for _, tt := range []struct {
		name string
		conn func(t *testing.T) *grpc.ClientConn
		want codes.Code
	}{
		{"weekly-summary with the right token", func(t *testing.T) *grpc.ClientConn { return internal(t, testInternalToken, "weekly-summary") }, codes.OK},
		{"wrong token", func(t *testing.T) *grpc.ClientConn { return internal(t, "wrong-token", "weekly-summary") }, codes.Unauthenticated},
		{"another internal caller", func(t *testing.T) *grpc.ClientConn { return internal(t, testInternalToken, "notification-dispatcher") }, codes.PermissionDenied},
		{"public client without a token", func(t *testing.T) *grpc.ClientConn { return dial(t, grpcclient.UnaryClientInterceptor()) }, codes.Unauthenticated},
	} {
		t.Run("ListUsersByRole: "+tt.name, func(t *testing.T) {
			resp, err := authv1.NewAuthInternalServiceClient(tt.conn(t)).ListUsersByRole(rpcCtx(t), listReq)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && len(resp.GetUsers()) != 1 {
				t.Fatalf("users %v, want one admin", resp.GetUsers())
			}
		})
	}

	t.Run("GetUser: notification-dispatcher allowed, weekly-summary denied", func(t *testing.T) {
		if _, err := authv1.NewAuthInternalServiceClient(internal(t, testInternalToken, "notification-dispatcher")).GetUser(rpcCtx(t), &authv1.GetUserRequest{}); err != nil {
			t.Fatalf("notification-dispatcher: %v", err)
		}
		_, err := authv1.NewAuthInternalServiceClient(internal(t, testInternalToken, "weekly-summary")).GetUser(rpcCtx(t), &authv1.GetUserRequest{})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("weekly-summary: code=%v, want PermissionDenied", status.Code(err))
		}
	})

	// Unimplemented, not Unauthenticated, shows the interceptor let it through.
	t.Run("public Login needs no internal token", func(t *testing.T) {
		_, err := authv1.NewAuthServiceClient(dial(t, grpcclient.UnaryClientInterceptor())).Login(rpcCtx(t), &authv1.LoginRequest{})
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("code=%v (%v), want Unimplemented", status.Code(err), err)
		}
	})
}
