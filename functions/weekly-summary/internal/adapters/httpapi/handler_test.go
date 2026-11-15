package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/adapters/httpapi"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/app"
)

type runnerFunc func(context.Context) (app.Result, error)

func (f runnerFunc) Run(ctx context.Context) (app.Result, error) { return f(ctx) }

func newHandler(run runnerFunc) (*httpapi.Handler, *bytes.Buffer) {
	var logs bytes.Buffer
	return &httpapi.Handler{Runner: run, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}, &logs
}

func TestHandler_ResultToStatus(t *testing.T) {
	for _, tt := range []struct {
		name       string
		result     app.Result
		err        error
		wantStatus int
		wantLog    string
	}{
		{"all sent", app.Result{Admins: 2, Sent: 2}, nil, http.StatusNoContent, `"msg":"weekly summary sent"`},
		{"no admins", app.Result{}, nil, http.StatusNoContent, `"admins":0`},
		{"analytics down", app.Result{}, fmt.Errorf("%w: private detail", app.ErrStats), http.StatusServiceUnavailable, `"reason":"statistics unavailable"`},
		{"auth down", app.Result{}, fmt.Errorf("%w: private detail", app.ErrAdmins), http.StatusServiceUnavailable, `"reason":"admin list unavailable"`},
		{"one send failed", app.Result{Admins: 2, Sent: 1, Failed: 1}, fmt.Errorf("%w for 1 of 2 admins", app.ErrSend), http.StatusServiceUnavailable, `"failed":1`},
		{"unknown error", app.Result{}, errors.New("private detail"), http.StatusServiceUnavailable, `"reason":"unexpected error"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, logs := newHandler(func(context.Context) (app.Result, error) { return tt.result, tt.err })
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"specversion":"1.0"}`)))
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Contains(t, logs.String(), tt.wantLog)
			assert.NotContains(t, logs.String(), "private")
			assert.NotContains(t, w.Body.String(), "private")
		})
	}
}

func TestHandler_RejectsBadRequestsWithoutRunning(t *testing.T) {
	for _, tt := range []struct {
		name   string
		req    *http.Request
		status int
	}{
		{"GET", httptest.NewRequest(http.MethodGet, "/", nil), http.StatusMethodNotAllowed},
		{"PUT", httptest.NewRequest(http.MethodPut, "/", nil), http.StatusMethodNotAllowed},
		{"body too large", httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", httpapi.MaxTriggerBody+1))), http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(func(context.Context) (app.Result, error) {
				t.Fatal("runner must not run")
				return app.Result{}, nil
			})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, tt.req)
			assert.Equal(t, tt.status, w.Code)
			if tt.status == http.StatusMethodNotAllowed {
				assert.Equal(t, http.MethodPost, w.Header().Get("Allow"))
			}
		})
	}
}

func TestHandler_BodyAtTheLimitIsAccepted(t *testing.T) {
	h, _ := newHandler(func(context.Context) (app.Result, error) { return app.Result{}, nil })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", httpapi.MaxTriggerBody))))
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandler_LogsTheCloudEventID(t *testing.T) {
	h, logs := newHandler(func(context.Context) (app.Result, error) { return app.Result{}, nil })
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Ce-Id", "ping-123")
	h.ServeHTTP(httptest.NewRecorder(), req)
	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	assert.Equal(t, "ping-123", entry["request_id"])
}

func TestHandler_RunHasADeadline(t *testing.T) {
	h, _ := newHandler(func(ctx context.Context) (app.Result, error) {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(httpapi.RunTimeout), deadline, 5*time.Second)
		return app.Result{}, nil
	})
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
}

func TestHandler_SecondRunWhileRunningIsRejected(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	h, _ := newHandler(func(context.Context) (app.Result, error) {
		close(started)
		<-release
		return app.Result{}, nil
	})
	var wg sync.WaitGroup
	first := httptest.NewRecorder()
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/", nil))
	}()
	<-started
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil))
	assert.Equal(t, http.StatusConflict, second.Code)
	close(release)
	wg.Wait()
	assert.Equal(t, http.StatusNoContent, first.Code)
}
