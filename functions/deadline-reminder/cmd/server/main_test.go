package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/adapters/httpapi"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
)

func validEnv() map[string]string {
	return map[string]string{
		"SERVICE_NAME":     "deadline-reminder",
		"JOB_POSTING_ADDR": "job-posting:9090",
		"SAVED_JOBS_ADDR":  "saved-jobs:9090",
		"RABBITMQ_URL":     "amqp://synthetic:private-secret@rabbitmq:5672/",
		"INTERNAL_TOKEN":   "test-internal-token",
	}
}

func TestLoadConfig(t *testing.T) {
	for _, tt := range []struct {
		name     string
		change   map[string]string
		want     config
		errField string
	}{
		{"defaults", nil, config{httpPort: "8080", jobPostingAddr: "job-posting:9090", savedJobsAddr: "saved-jobs:9090",
			rabbitMQURL: "amqp://synthetic:private-secret@rabbitmq:5672/", internalToken: "test-internal-token"}, ""},
		{"custom port and trimmed values", map[string]string{"HTTP_PORT": "18080", "JOB_POSTING_ADDR": " job-posting:9090 ", "SAVED_JOBS_ADDR": "\tsaved-jobs:9090", "RABBITMQ_URL": " amqps://synthetic:private-secret@rabbitmq:5671/ "},
			config{httpPort: "18080", jobPostingAddr: "job-posting:9090", savedJobsAddr: "saved-jobs:9090",
				rabbitMQURL: "amqps://synthetic:private-secret@rabbitmq:5671/", internalToken: "test-internal-token"}, ""},
		{"lowest port", map[string]string{"HTTP_PORT": "1"}, config{}, ""},
		{"highest port", map[string]string{"HTTP_PORT": "65535"}, config{}, ""},
		{"missing service name", map[string]string{"SERVICE_NAME": ""}, config{}, "SERVICE_NAME"},
		{"other service name", map[string]string{"SERVICE_NAME": "weekly-summary"}, config{}, "SERVICE_NAME"},
		{"zero port", map[string]string{"HTTP_PORT": "0"}, config{}, "HTTP_PORT"},
		{"port too high", map[string]string{"HTTP_PORT": "65536"}, config{}, "HTTP_PORT"},
		{"port not a number", map[string]string{"HTTP_PORT": "private-secret"}, config{}, "HTTP_PORT"},
		{"missing job-posting", map[string]string{"JOB_POSTING_ADDR": " "}, config{}, "JOB_POSTING_ADDR"},
		{"missing saved-jobs", map[string]string{"SAVED_JOBS_ADDR": ""}, config{}, "SAVED_JOBS_ADDR"},
		{"missing RabbitMQ URL", map[string]string{"RABBITMQ_URL": ""}, config{}, "RABBITMQ_URL"},
		{"RabbitMQ URL without scheme", map[string]string{"RABBITMQ_URL": "synthetic:private-secret@rabbitmq:5672"}, config{}, "RABBITMQ_URL"},
		{"RabbitMQ URL with another scheme", map[string]string{"RABBITMQ_URL": "http://synthetic:private-secret@rabbitmq:5672/"}, config{}, "RABBITMQ_URL"},
		{"RabbitMQ URL without host", map[string]string{"RABBITMQ_URL": "amqp:///private-secret"}, config{}, "RABBITMQ_URL"},
		{"missing internal token", map[string]string{"INTERNAL_TOKEN": ""}, config{}, "INTERNAL_TOKEN"},
		{"internal token with space", map[string]string{"INTERNAL_TOKEN": "private-secret value"}, config{}, "INTERNAL_TOKEN"},
		{"non-ASCII internal token", map[string]string{"INTERNAL_TOKEN": "private-secreté"}, config{}, "INTERNAL_TOKEN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			for k, v := range tt.change {
				env[k] = v
			}
			got, err := loadConfig(func(key string) string { return env[key] })
			if tt.errField != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errField)
				assert.NotContains(t, err.Error(), "private-secret")
				return
			}
			require.NoError(t, err)
			if tt.want != (config{}) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDialInternal_RejectsAnInvalidTokenWithoutLeakingIt(t *testing.T) {
	_, err := dialInternal("job-posting:9090", "private-secret value")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private-secret")

	conn, err := dialInternal("job-posting:9090", "test-internal-token")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestNewMux_Routes(t *testing.T) {
	triggered := 0
	mux := newMux(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		triggered++
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tt := range []struct {
		method, path string
		status       int
		triggers     int
	}{
		{http.MethodPost, "/", http.StatusNoContent, 1},
		{http.MethodGet, "/healthz", http.StatusOK, 0},
		{http.MethodGet, "/metrics", http.StatusOK, 0},
		{http.MethodPost, "/remind", http.StatusNotFound, 0},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed, 0},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			before := triggered
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			assert.Equal(t, tt.status, w.Code)
			assert.Equal(t, tt.triggers, triggered-before)
			if tt.path == "/metrics" {
				assert.Equal(t, "jobtrack_deadline_reminder_up 1\n", w.Body.String())
			}
		})
	}
}

func TestNewServer_TimeoutsCoverAFullRun(t *testing.T) {
	s := newServer(":8080", http.NewServeMux())
	assert.Greater(t, s.WriteTimeout, httpapi.RunTimeout)
	assert.Equal(t, 5*time.Second, s.ReadHeaderTimeout)
	assert.NotZero(t, s.ReadTimeout)
	assert.NotZero(t, s.IdleTimeout)
}

func TestRun_StopsOnShutdownAndOnBindFailure(t *testing.T) {
	free, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(free.Addr().(*net.TCPAddr).Port)
	require.NoError(t, free.Close())
	cfg, err := loadConfig(func(key string) string {
		if key == "HTTP_PORT" {
			return port
		}
		return validEnv()[key]
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, logging.New(serviceName, io.Discard)) }()

	// Wait until the function answers, then stop it like SIGTERM would.
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	require.Eventually(t, func() bool {
		resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 50*time.Millisecond)

	// A second instance on the same port fails instead of hanging.
	err = run(context.Background(), cfg, logging.New(serviceName, io.Discard))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "address already in use") || strings.Contains(err.Error(), "bind"), err.Error())

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(ShutdownTimeout + 5*time.Second):
		t.Fatal("run did not stop")
	}
}
