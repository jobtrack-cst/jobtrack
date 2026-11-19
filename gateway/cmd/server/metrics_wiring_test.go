package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jobtrack-cst/jobtrack/gateway/internal/jwtauth"
	"github.com/stretchr/testify/require"
)

func TestRequestMetrics_IncludeAuthenticationFailuresAndPreserveReadiness(t *testing.T) {
	verifier, err := jwtauth.NewVerifier(testSecret, nil)
	require.NoError(t, err)
	calls := 0
	h := newHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(201) }), verifier, discardLogger())
	for _, path := range []string{"/v1/secret-user-id?token=private", "/v1/auth/login"} {
		method := "GET"
		expected := 401
		if path == "/v1/auth/login" {
			method = "POST"
			expected = 201
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		require.Equal(t, expected, rec.Code)
	}
	require.Equal(t, 1, calls)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
		require.Equal(t, 200, rec.Code)
		body := rec.Body.String()
		require.Contains(t, body, "jobtrack_service_ready 1")
		require.Contains(t, body, `result="error",status="401"} 1`)
		require.Contains(t, body, `result="success",status="201"} 1`)
		require.Contains(t, body, `jobtrack_request_duration_seconds_count{service="gateway",method="api"} 2`)
		require.NotContains(t, body, "secret-user-id")
		require.NotContains(t, body, "private")
	}
}
