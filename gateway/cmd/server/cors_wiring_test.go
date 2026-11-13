package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/backends"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/cors"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
)

const corsDevOrigin = "http://localhost:5173"

func TestLoadConfig_CORSAllowedOrigin(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantOrigin string
		wantErr    bool
	}{
		{name: "unset turns CORS off", value: ""},
		{name: "Vite dev server", value: corsDevOrigin, wantOrigin: corsDevOrigin},
		{name: "wildcard refused", value: "*", wantErr: true},
		{name: "trailing slash refused", value: corsDevOrigin + "/", wantErr: true},
		{name: "no scheme refused", value: "localhost:5173", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			if tt.value != "" {
				env["CORS_ALLOWED_ORIGIN"] = tt.value
			}
			cfg, err := loadConfig(func(key string) string { return env[key] })
			if tt.wantErr {
				require.ErrorIs(t, err, cors.ErrInvalidOrigin)
				require.ErrorContains(t, err, "CORS_ALLOWED_ORIGIN")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOrigin, cfg.corsPolicy.Origin())
		})
	}
}

// corsAuthBackend records the metadata of every GetCurrentUser call it receives.
type corsAuthBackend struct {
	authv1.UnimplementedAuthServiceServer
	calls chan metadata.MD
}

func (b *corsAuthBackend) GetCurrentUser(ctx context.Context, _ *authv1.GetCurrentUserRequest) (*authv1.GetCurrentUserResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	b.calls <- md
	return &authv1.GetCurrentUserResponse{}, nil
}

// newCORSGateway builds the same handler chain as run, with a real auth backend
// on a local port and every other backend unreachable.
func newCORSGateway(t *testing.T) (http.Handler, *corsAuthBackend) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	backend := &corsAuthBackend{calls: make(chan metadata.MD, 10)}
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, backend)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	addrs := map[string]string{}
	for _, b := range backends.All() {
		addrs[b.Name] = "127.0.0.1:1"
	}
	addrs["auth"] = listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	api, closeConns, err := newAPI(ctx, addrs)
	require.NoError(t, err)
	t.Cleanup(closeConns)

	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	policy, err := cors.Parse(corsDevOrigin)
	require.NoError(t, err)
	return policy.Wrap(newHandler(api, verifier, discardLogger())), backend
}

func corsTestToken(t *testing.T) string {
	t.Helper()
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  testUserID,
		"role": "applicant",
		"iss":  jwtauth.Issuer,
		"iat":  now.Unix(),
		"exp":  now.Add(time.Hour).Unix(),
	}).SignedString([]byte(testSecret))
	require.NoError(t, err)
	return token
}

func TestGateway_CORS(t *testing.T) {
	handler, backend := newCORSGateway(t)
	token := corsTestToken(t)

	tests := []struct {
		name         string
		method       string
		path         string
		headers      map[string]string
		wantCode     int
		wantAllow    bool
		wantBackend  bool
		wantIdentity bool
	}{
		{
			name: "preflight for /v1/auth/me needs no JWT", method: http.MethodOptions, path: "/v1/auth/me",
			headers: map[string]string{
				"Origin": corsDevOrigin, "Access-Control-Request-Method": "GET",
				"Access-Control-Request-Headers": "authorization",
			},
			wantCode: http.StatusNoContent, wantAllow: true,
		},
		{
			name: "preflight for login needs no JWT", method: http.MethodOptions, path: "/v1/auth/login",
			headers: map[string]string{
				"Origin": corsDevOrigin, "Access-Control-Request-Method": "POST",
				"Access-Control-Request-Headers": "content-type",
			},
			wantCode: http.StatusNoContent, wantAllow: true,
		},
		{
			name: "preflight from another origin is refused", method: http.MethodOptions, path: "/v1/auth/me",
			headers:  map[string]string{"Origin": "http://evil.example", "Access-Control-Request-Method": "GET"},
			wantCode: http.StatusForbidden,
		},
		{
			name: "allowed origin without a token is still 401", method: http.MethodGet, path: "/v1/auth/me",
			headers:  map[string]string{"Origin": corsDevOrigin},
			wantCode: http.StatusUnauthorized, wantAllow: true,
		},
		{
			name: "OPTIONS that is not a preflight still needs a JWT", method: http.MethodOptions, path: "/v1/auth/me",
			headers:  map[string]string{"Origin": corsDevOrigin},
			wantCode: http.StatusUnauthorized, wantAllow: true,
		},
		{
			name: "allowed origin with a token reaches auth, spoofed headers stripped", method: http.MethodGet, path: "/v1/auth/me",
			headers: map[string]string{
				"Origin": corsDevOrigin, "Authorization": "Bearer " + token,
				"Grpc-Metadata-X-User-Role": "admin", "Grpc-Metadata-X-Internal-Token": "stolen",
			},
			wantCode: http.StatusOK, wantAllow: true, wantBackend: true, wantIdentity: true,
		},
		{
			// CORS is enforced by the browser; the server only withholds the allow header.
			name: "other origin with a token is served without the allow header", method: http.MethodGet, path: "/v1/auth/me",
			headers:  map[string]string{"Origin": "http://evil.example", "Authorization": "Bearer " + token},
			wantCode: http.StatusOK, wantBackend: true, wantIdentity: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req := httptest.NewRequest(tt.method, tt.path, nil).WithContext(ctx)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.wantCode, rec.Code, rec.Body.String())
			if tt.wantAllow {
				require.Equal(t, corsDevOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
			select {
			case md := <-backend.calls:
				require.True(t, tt.wantBackend, "backend must not be called")
				if tt.wantIdentity {
					require.Equal(t, []string{testUserID}, md.Get("x-user-id"))
					require.Equal(t, []string{"applicant"}, md.Get("x-user-role"))
					require.Empty(t, md.Get("x-internal-token"))
				}
			default:
				require.False(t, tt.wantBackend, "backend was not called")
			}
		})
	}
}
