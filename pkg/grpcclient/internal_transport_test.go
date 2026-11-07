package grpcclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type internalAuthServer struct {
	authv1.UnimplementedAuthInternalServiceServer
}

func (internalAuthServer) GetUser(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	return &authv1.GetUserResponse{}, nil
}
func (internalAuthServer) ListUsersByRole(context.Context, *authv1.ListUsersByRoleRequest) (*authv1.ListUsersByRoleResponse, error) {
	return &authv1.ListUsersByRoleResponse{}, nil
}

func TestInternalInterceptors_GeneratedContractTransport(t *testing.T) {
	const token = "synthetic-jt44-transport-token"
	serverAuth, err := authctx.InternalUnaryServerInterceptor(token)
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(serverAuth))
	authv1.RegisterAuthInternalServiceServer(server, internalAuthServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	tests := []struct {
		name, caller, token string
		list                bool
		code                codes.Code
	}{
		{"allowed get", "notification-dispatcher", token, false, codes.OK},
		{"allowed list", "weekly-summary", token, true, codes.OK},
		{"wrong rpc", "weekly-summary", token, false, codes.PermissionDenied},
		{"wrong token", "notification-dispatcher", "synthetic-wrong-token", false, codes.Unauthenticated},
		{"unconfigured", "", "", false, codes.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := grpcclient.UnaryClientInterceptor()
			if tt.caller != "" {
				var err error
				interceptor, err = grpcclient.InternalUnaryClientInterceptor(tt.token, tt.caller)
				if err != nil {
					t.Fatal(err)
				}
			}
			conn, err := grpc.NewClient("passthrough:///internal-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithChainUnaryInterceptor(interceptor))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tt.caller != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "x-internal-token", "spoofed", "x-caller-service", "spoofed")
			}
			client := authv1.NewAuthInternalServiceClient(conn)
			if tt.list {
				_, err = client.ListUsersByRole(ctx, &authv1.ListUsersByRoleRequest{})
			} else {
				_, err = client.GetUser(ctx, &authv1.GetUserRequest{})
			}
			if status.Code(err) != tt.code {
				t.Fatalf("code=%v want=%v", status.Code(err), tt.code)
			}
		})
	}
}
