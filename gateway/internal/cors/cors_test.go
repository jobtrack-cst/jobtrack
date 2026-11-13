package cors_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/cors"
)

const (
	devOrigin = "http://localhost:5173"
	routeMe   = "/v1/auth/me"
)

func TestParse_AcceptsOrigins(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "Vite dev server", value: devOrigin},
		{name: "loopback address", value: "http://127.0.0.1:5173"},
		{name: "https without port", value: "https://jobtrack.example"},
		{name: "IPv6 loopback", value: "http://[::1]:5173"},
		{name: "port 1", value: "http://localhost:1"},
		{name: "port 65535", value: "http://localhost:65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := cors.Parse(tt.value)
			require.NoError(t, err)
			require.Equal(t, tt.value, p.Origin())
		})
	}
}

func TestParse_EmptyTurnsCORSOff(t *testing.T) {
	p, err := cors.Parse("")
	require.NoError(t, err)
	require.Empty(t, p.Origin())
}

func TestParse_RejectsInvalidOrigins(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "wildcard", value: "*"},
		{name: "null origin", value: "null"},
		{name: "no scheme", value: "localhost:5173"},
		{name: "ftp scheme", value: "ftp://localhost:5173"},
		{name: "no host", value: "http://"},
		{name: "trailing slash", value: devOrigin + "/"},
		{name: "path", value: devOrigin + "/app"},
		{name: "query", value: devOrigin + "?a=b"},
		{name: "fragment", value: devOrigin + "#top"},
		{name: "user info", value: "http://user@localhost:5173"},
		{name: "uppercase scheme", value: "HTTP://localhost:5173"},
		{name: "uppercase host", value: "http://LOCALHOST:5173"},
		{name: "leading space", value: " " + devOrigin},
		{name: "trailing space", value: devOrigin + " "},
		{name: "empty port", value: "http://localhost:"},
		{name: "port 0", value: "http://localhost:0"},
		{name: "port 65536", value: "http://localhost:65536"},
		{name: "two origins", value: devOrigin + ",http://127.0.0.1:5173"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cors.Parse(tt.value)
			require.ErrorIs(t, err, cors.ErrInvalidOrigin)
		})
	}
}

func mustPolicy(t *testing.T) cors.Policy {
	t.Helper()
	p, err := cors.Parse(devOrigin)
	require.NoError(t, err)
	return p
}

// serve runs the request through the policy and reports whether next was reached.
func serve(t *testing.T, p cors.Policy, req *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot) // stands in for whatever the gateway answers
	})
	rec := httptest.NewRecorder()
	p.Wrap(next).ServeHTTP(rec, req)
	return rec, called
}

func TestWrap_OtherRequests(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		origins   []string
		wantAllow bool
	}{
		{name: "allowed origin GET", method: http.MethodGet, origins: []string{devOrigin}, wantAllow: true},
		{name: "allowed origin POST", method: http.MethodPost, origins: []string{devOrigin}, wantAllow: true},
		{name: "other origin", method: http.MethodGet, origins: []string{"http://evil.example"}},
		{name: "other port", method: http.MethodGet, origins: []string{"http://localhost:3000"}},
		{name: "127.0.0.1 is a different origin", method: http.MethodGet, origins: []string{"http://127.0.0.1:5173"}},
		{name: "origin with trailing slash", method: http.MethodGet, origins: []string{devOrigin + "/"}},
		{name: "uppercase origin", method: http.MethodGet, origins: []string{"http://LOCALHOST:5173"}},
		{name: "null origin", method: http.MethodGet, origins: []string{"null"}},
		{name: "no Origin header", method: http.MethodGet},
		{name: "two Origin headers", method: http.MethodGet, origins: []string{devOrigin, "http://evil.example"}},
		// Not a preflight, so it must reach the JWT middleware like any other request.
		{name: "OPTIONS without request method", method: http.MethodOptions, origins: []string{devOrigin}, wantAllow: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, routeMe, nil)
			for _, o := range tt.origins {
				req.Header.Add("Origin", o)
			}

			rec, called := serve(t, mustPolicy(t), req)

			require.True(t, called, "non-preflight requests must reach next")
			require.Equal(t, http.StatusTeapot, rec.Code)
			require.Contains(t, rec.Header().Values("Vary"), "Origin")
			if tt.wantAllow {
				require.Equal(t, devOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
			require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func TestWrap_Preflight(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		requestMethods []string
		requestHeaders []string
		wantCode       int
	}{
		{name: "login POST with content-type", origin: devOrigin,
			requestMethods: []string{"POST"}, requestHeaders: []string{"content-type"}, wantCode: http.StatusNoContent},
		{name: "me GET with authorization", origin: devOrigin,
			requestMethods: []string{"GET"}, requestHeaders: []string{"authorization"}, wantCode: http.StatusNoContent},
		{name: "both headers, mixed case and spaces", origin: devOrigin,
			requestMethods: []string{"PUT"}, requestHeaders: []string{" Authorization , Content-Type "}, wantCode: http.StatusNoContent},
		{name: "headers on two lines", origin: devOrigin,
			requestMethods: []string{"PATCH"}, requestHeaders: []string{"authorization", "content-type"}, wantCode: http.StatusNoContent},
		{name: "DELETE without extra headers", origin: devOrigin,
			requestMethods: []string{"DELETE"}, wantCode: http.StatusNoContent},
		{name: "other origin", origin: "http://evil.example",
			requestMethods: []string{"GET"}, requestHeaders: []string{"authorization"}, wantCode: http.StatusForbidden},
		{name: "no Origin header",
			requestMethods: []string{"GET"}, wantCode: http.StatusForbidden},
		{name: "method not used by the API", origin: devOrigin,
			requestMethods: []string{"TRACE"}, wantCode: http.StatusForbidden},
		{name: "lowercase method", origin: devOrigin,
			requestMethods: []string{"get"}, wantCode: http.StatusForbidden},
		{name: "empty request method", origin: devOrigin,
			requestMethods: []string{""}, wantCode: http.StatusForbidden},
		{name: "two request methods", origin: devOrigin,
			requestMethods: []string{"GET", "POST"}, wantCode: http.StatusForbidden},
		{name: "identity header", origin: devOrigin,
			requestMethods: []string{"GET"}, requestHeaders: []string{"authorization, x-user-role"}, wantCode: http.StatusForbidden},
		{name: "internal token header", origin: devOrigin,
			requestMethods: []string{"GET"}, requestHeaders: []string{"grpc-metadata-x-internal-token"}, wantCode: http.StatusForbidden},
		{name: "method override header", origin: devOrigin,
			requestMethods: []string{"POST"}, requestHeaders: []string{"x-http-method-override"}, wantCode: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, routeMe, nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			for _, m := range tt.requestMethods {
				req.Header.Add("Access-Control-Request-Method", m)
			}
			for _, h := range tt.requestHeaders {
				req.Header.Add("Access-Control-Request-Headers", h)
			}

			rec, called := serve(t, mustPolicy(t), req)

			require.False(t, called, "a preflight must never reach authentication or a backend")
			require.Equal(t, tt.wantCode, rec.Code)
			require.Contains(t, rec.Header().Values("Vary"), "Origin")
			if tt.wantCode == http.StatusNoContent {
				require.Equal(t, devOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
				require.Equal(t, "GET, POST, PUT, PATCH, DELETE", rec.Header().Get("Access-Control-Allow-Methods"))
				require.Equal(t, "Authorization, Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
				require.Equal(t, "600", rec.Header().Get("Access-Control-Max-Age"))
			} else {
				require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
				require.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
			}
			require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func TestWrap_OffPassesEverythingThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, routeMe, nil)
	req.Header.Set("Origin", devOrigin)
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec, called := serve(t, cors.Policy{}, req)

	require.True(t, called, "with CORS off the gateway behaves exactly as before")
	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, rec.Header().Values("Vary"))
}
