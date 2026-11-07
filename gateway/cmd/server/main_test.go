package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/backends"
	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
)

const (
	testSecret = "0123456789abcdef0123456789abcdef" // exactly 32 bytes
	testUserID = "3f2b8c1e-7a4d-4c2b-9e1f-0a1b2c3d4e5f"
)

func validEnv() map[string]string {
	env := map[string]string{"SERVICE_NAME": "gateway", "JWT_SECRET": testSecret}
	for _, b := range backends.All() {
		env[b.AddrEnv()] = b.Name + ":9090"
	}
	return env
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		change   func(map[string]string)
		wantErr  string
		wantPort int
	}{
		{name: "valid with default port", change: func(map[string]string) {}, wantPort: 8080},
		{name: "custom HTTP_PORT", change: func(e map[string]string) { e["HTTP_PORT"] = "9000" }, wantPort: 9000},
		{name: "missing SERVICE_NAME", change: func(e map[string]string) { delete(e, "SERVICE_NAME") }, wantErr: "SERVICE_NAME"},
		{name: "HTTP_PORT not a number", change: func(e map[string]string) { e["HTTP_PORT"] = "abc" }, wantErr: "HTTP_PORT"},
		{name: "HTTP_PORT zero", change: func(e map[string]string) { e["HTTP_PORT"] = "0" }, wantErr: "HTTP_PORT"},
		{name: "HTTP_PORT too large", change: func(e map[string]string) { e["HTTP_PORT"] = "65536" }, wantErr: "HTTP_PORT"},
		{name: "missing JWT_SECRET", change: func(e map[string]string) { delete(e, "JWT_SECRET") }, wantErr: "JWT_SECRET"},
		{name: "JWT_SECRET 31 bytes", change: func(e map[string]string) { e["JWT_SECRET"] = strings.Repeat("a", 31) }, wantErr: "JWT_SECRET"},
		{name: "INTERNAL_TOKEN set", change: func(e map[string]string) { e["INTERNAL_TOKEN"] = "secret" }, wantErr: "INTERNAL_TOKEN"},
		{name: "missing AUTH_ADDR", change: func(e map[string]string) { delete(e, "AUTH_ADDR") }, wantErr: "AUTH_ADDR"},
		{name: "blank SAVED_JOBS_ADDR", change: func(e map[string]string) { e["SAVED_JOBS_ADDR"] = "  " }, wantErr: "SAVED_JOBS_ADDR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			tt.change(env)
			cfg, err := loadConfig(func(key string) string { return env[key] })
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantPort, cfg.httpPort)
			require.Len(t, cfg.addrs, 20)
		})
	}
}

func TestNewHandler_Routes(t *testing.T) {
	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	tests := []struct {
		name        string
		method      string
		path        string
		wantCode    int
		wantBody    string
		wantAPICall bool
	}{
		{name: "healthz without token", method: http.MethodGet, path: "/healthz", wantCode: http.StatusOK, wantBody: "ok\n"},
		{name: "metrics without token", method: http.MethodGet, path: "/metrics", wantCode: http.StatusOK, wantBody: "jobtrack_service_ready 1"},
		{name: "protected route without token", method: http.MethodGet, path: "/v1/auth/me", wantCode: http.StatusUnauthorized},
		{name: "POST healthz is not exempt", method: http.MethodPost, path: "/healthz", wantCode: http.StatusUnauthorized},
		{name: "login reaches the API without token", method: http.MethodPost, path: "/v1/auth/login", wantCode: http.StatusOK, wantAPICall: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			rec := httptest.NewRecorder()
			newHandler(api, verifier, discardLogger()).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			require.Equal(t, tt.wantCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.wantBody)
			require.Equal(t, tt.wantAPICall, called)
		})
	}
}

// fakeAuth records the metadata that reaches the auth service.
type fakeAuth struct {
	authv1.UnimplementedAuthServiceServer
	received chan metadata.MD
}

func (f *fakeAuth) GetCurrentUser(ctx context.Context, _ *authv1.GetCurrentUserRequest) (*authv1.GetCurrentUserResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.received <- md
	return &authv1.GetCurrentUserResponse{}, nil
}

// TestGateway_ForwardsVerifiedIdentityToBackend runs the real wiring (backends list,
// pkg/grpcclient, middleware) against a fake auth gRPC server.
func TestGateway_ForwardsVerifiedIdentityToBackend(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	fake := &fakeAuth{received: make(chan metadata.MD, 1)}
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	addrs := map[string]string{}
	for _, b := range backends.All() {
		addrs[b.Name] = "127.0.0.1:1" // never called in this test
	}
	addrs["auth"] = listener.Addr().String()

	api, closeConns, err := newAPI(context.Background(), addrs)
	require.NoError(t, err)
	t.Cleanup(closeConns)
	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	handler := newHandler(api, verifier, discardLogger())

	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": testUserID, "role": "applicant", "iss": jwtauth.Issuer,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString([]byte(testSecret))
	require.NoError(t, err)

	t.Run("valid token with spoofed headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Grpc-Metadata-X-User-Id", "attacker")
		req.Header.Set("Grpc-Metadata-X-User-Role", "admin")
		req.Header.Set("Grpc-Metadata-X-Internal-Token", "stolen")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		select {
		case md := <-fake.received:
			require.Equal(t, []string{testUserID}, md.Get("x-user-id"))
			require.Equal(t, []string{"applicant"}, md.Get("x-user-role"))
			require.Empty(t, md.Get("x-internal-token"))
		case <-time.After(2 * time.Second):
			t.Fatal("auth service was not called")
		}
	})

	t.Run("missing token never reaches the backend", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		select {
		case <-fake.received:
			t.Fatal("backend was called without a token")
		default:
		}
	})
}
