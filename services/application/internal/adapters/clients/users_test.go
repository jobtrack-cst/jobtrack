package clients_test

import (
	"context"
	"net"
	"strings"
	"testing"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/clients"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const internalToken = "synthetic-jt114-token"

type authServer struct {
	authv1.UnimplementedAuthInternalServiceServer
	get func(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error)
}

func (s authServer) GetUser(c context.Context, r *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	return s.get(c, r)
}

// authConnection serves auth behind the real JT-44 server interceptor and dials
// it with the internal client interceptor, as main wires the connection.
func authConnection(t *testing.T, srv authServer, serverToken, caller string) *grpc.ClientConn {
	t.Helper()
	guard, err := authctx.InternalUnaryServerInterceptor(serverToken)
	require.NoError(t, err)
	l := bufconn.Listen(1 << 20)
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(guard))
	authv1.RegisterAuthInternalServiceServer(s, srv)
	go s.Serve(l)
	t.Cleanup(s.Stop)
	internal, err := grpcclient.InternalUnaryClientInterceptor(internalToken, caller)
	require.NoError(t, err)
	c, err := grpc.NewClient("passthrough:///auth", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return l.Dial() }), grpc.WithChainUnaryInterceptor(internal))
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func user(userID, name string) *authv1.GetUserResponse {
	return &authv1.GetUserResponse{User: &authv1.User{UserId: userID, Email: "synthetic@example.test", FullName: name, Role: authv1.Role_ROLE_APPLICANT}}
}

func TestUsers_GetUserName_SendsInternalCredentialsNotUserIdentity(t *testing.T) {
	c := authConnection(t, authServer{get: func(ctx context.Context, r *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
		require.Equal(t, owner, r.GetUserId())
		m, _ := metadata.FromIncomingContext(ctx)
		require.Equal(t, []string{"application"}, m.Get("x-caller-service"))
		// The applicant's identity is not forwarded to an internal RPC.
		require.Empty(t, m.Get("x-user-id"))
		require.Empty(t, m.Get("x-user-role"))
		return user(owner, "Pema Dorji"), nil
	}}, internalToken, "application")
	name, err := clients.NewUsers(authv1.NewAuthInternalServiceClient(c)).GetUserName(caller(), owner)
	require.NoError(t, err)
	require.Equal(t, "Pema Dorji", name)
}

func TestUsers_GetUserName_RejectsInvalidResponses(t *testing.T) {
	for _, tt := range []struct {
		name string
		resp *authv1.GetUserResponse
	}{
		{"missing user", &authv1.GetUserResponse{}},
		{"another user", user(id, "Pema Dorji")},
		{"empty name", user(owner, "")},
		{"blank name", user(owner, "   ")},
		{"201 characters", user(owner, strings.Repeat("a", 201))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := authConnection(t, authServer{get: func(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
				return tt.resp, nil
			}}, internalToken, "application")
			name, err := clients.NewUsers(authv1.NewAuthInternalServiceClient(c)).GetUserName(caller(), owner)
			require.Equal(t, codes.Internal, status.Code(err))
			require.NotContains(t, err.Error(), "aaaa")
			require.Empty(t, name)
		})
	}
	t.Run("200 characters accepted", func(t *testing.T) {
		long := strings.Repeat("ཀ", 200)
		c := authConnection(t, authServer{get: func(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
			return user(owner, long), nil
		}}, internalToken, "application")
		name, err := clients.NewUsers(authv1.NewAuthInternalServiceClient(c)).GetUserName(caller(), owner)
		require.NoError(t, err)
		require.Equal(t, long, name)
	})
}

func TestUsers_GetUserName_ErrorsAndRetries(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.Unavailable, codes.Internal, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			calls := 0
			c := authConnection(t, authServer{get: func(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
				calls++
				return nil, status.Error(code, "private dependency error")
			}}, internalToken, "application")
			_, err := clients.NewUsers(authv1.NewAuthInternalServiceClient(c)).GetUserName(caller(), owner)
			require.Equal(t, code, status.Code(err))
			want := 1
			if code == codes.Unavailable {
				want = 4 // grpcclient retries UNAVAILABLE up to 3 times.
			}
			require.Equal(t, want, calls)
		})
	}
	t.Run("wrong token is rejected by auth", func(t *testing.T) {
		c := authConnection(t, authServer{get: func(context.Context, *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
			t.Fatal("handler reached with a wrong token")
			return nil, nil
		}}, "another-synthetic-token", "application")
		_, err := clients.NewUsers(authv1.NewAuthInternalServiceClient(c)).GetUserName(caller(), owner)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})
}
