package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

func TestMetrics_ReadinessExposition(t *testing.T) {
	for _, tt := range []struct {
		name   string
		health domain.HealthStatus
		err    error
		value  string
	}{
		{"healthy", domain.Serving, nil, "1"},
		{"unhealthy", domain.NotServing, nil, "0"},
		{"unknown", domain.Unknown, nil, "0"},
		{"invalid status", domain.HealthStatus(255), nil, "0"},
		{"checker error", domain.Unknown, errors.New("private database credentials"), "0"},
		{"error overrides healthy status", domain.Serving, errors.New("private database credentials"), "0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
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
			if response.Code != http.StatusOK || calls != 1 {
				t.Fatalf("status=%d calls=%d", response.Code, calls)
			}
			if got := response.Header().Get("Content-Type"); got != "text/plain; version=0.0.4; charset=utf-8" {
				t.Fatalf("unexpected Content-Type %q", got)
			}
			want := "# HELP jobtrack_service_ready Whether the service readiness check succeeds.\n" +
				"# TYPE jobtrack_service_ready gauge\n" + "jobtrack_service_ready " + tt.value + "\n"
			if got := response.Body.String(); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestHTTPRoutes_RejectUnsupportedMethods(t *testing.T) {
	handler := healthHandler(checkerFunc(func(context.Context) (domain.HealthStatus, error) {
		t.Error("unsupported method reached checker")
		return domain.Serving, nil
	}))
	for _, path := range []string{"/metrics", "/healthz"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			t.Run(method+path, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
				if response.Code != http.StatusMethodNotAllowed {
					t.Fatalf("status=%d, want 405", response.Code)
				}
			})
		}
	}
}
