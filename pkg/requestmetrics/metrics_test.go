package requestmetrics_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/requestmetrics"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func exposition(m *requestmetrics.Recorder) string {
	var b bytes.Buffer
	m.WritePrometheus(&b)
	return b.String()
}
func recorder() *requestmetrics.Recorder {
	m := requestmetrics.New("auth")
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, &authv1.UnimplementedAuthServiceServer{})
	authv1.RegisterAuthInternalServiceServer(server, &authv1.UnimplementedAuthInternalServiceServer{})
	m.RegisterGRPC(server)
	return m
}

func TestUnary_RecordsResultsDurationAndBoundedMethods(t *testing.T) {
	m := recorder()
	intercept := m.UnaryServerInterceptor()
	method := "/jobtrack.auth.v1.AuthService/Login"
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"success", nil, "OK"}, {"failure", status.Error(codes.InvalidArgument, "private-error-user-data"), "InvalidArgument"},
		{"canceled", context.Canceled, "Canceled"},
		{"deadline", context.DeadlineExceeded, "DeadlineExceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply := new(int)
			got, err := intercept(context.Background(), "private-request", &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, req any) (any, error) {
				require.Equal(t, "private-request", req)
				time.Sleep(5 * time.Millisecond)
				return reply, tc.err
			})
			require.Same(t, reply, got)
			require.Equal(t, tc.err, err)
			require.Contains(t, exposition(m), fmt.Sprintf("status=%q} 1", tc.code))
		})
	}
	for i := 0; i < 100; i++ {
		_, err := intercept(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: fmt.Sprintf("/private-user-id/%d", i)}, func(context.Context, any) (any, error) { return nil, nil })
		require.NoError(t, err)
	}
	before := exposition(m)
	_, err := intercept(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, func(context.Context, any) (any, error) { return nil, nil })
	require.NoError(t, err)
	require.Equal(t, before, exposition(m))
	require.NotContains(t, before, "private-")
	require.Contains(t, before, `method="unknown",result="success",status="OK"} 100`)
	require.Contains(t, before, `jobtrack_request_duration_seconds_count{service="auth",method="`+method+`"} 4`)
	require.Contains(t, before, `method="`+method+`",le="+Inf"} 4`)
	prefix := `jobtrack_request_duration_seconds_sum{service="auth",method="` + method + `"} `
	for _, line := range strings.Split(before, "\n") {
		if strings.HasPrefix(line, prefix) {
			seconds, err := strconv.ParseFloat(strings.TrimPrefix(line, prefix), 64)
			require.NoError(t, err)
			require.GreaterOrEqual(t, seconds, .010)
		}
	}
}

func TestUnary_ObservesInternalAuthenticationRejection(t *testing.T) {
	m := recorder()
	authenticate, err := authctx.InternalUnaryServerInterceptor("synthetic-internal-token")
	require.NoError(t, err)
	called := false
	info := &grpc.UnaryServerInfo{FullMethod: "/jobtrack.auth.v1.AuthInternalService/GetUser"}
	_, err = m.UnaryServerInterceptor()(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return authenticate(ctx, req, info, func(context.Context, any) (any, error) { called = true; return nil, nil })
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, called)
	require.Contains(t, exposition(m), `result="error",status="Unauthenticated"} 1`)
	require.NotContains(t, exposition(m), "synthetic-internal-token")
}

func TestHTTP_StatusTimingExclusionsSafeLabelsAndInterfaces(t *testing.T) {
	m := requestmetrics.New("gateway")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Implements(t, (*http.Flusher)(nil), w)
		time.Sleep(time.Millisecond)
		switch r.URL.Path {
		case "/private-id":
			w.WriteHeader(103)
			w.WriteHeader(403)
			w.WriteHeader(500)
		case "/failure":
			http.Error(w, "private body", 500)
		default:
			fmt.Fprint(w, "ok")
		}
	})
	h := m.HTTP(next)
	for _, path := range []string{"/private-id?token=secret", "/failure", "/success"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	before := exposition(m)
	for _, path := range []string{"/healthz", "/metrics"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	require.Equal(t, before, exposition(m))
	require.Contains(t, before, `result="error",status="403"} 1`)
	require.Contains(t, before, `result="error",status="500"} 1`)
	require.Contains(t, before, `result="success",status="200"} 1`)
	require.Contains(t, before, `jobtrack_request_duration_seconds_count{service="gateway",method="api"} 3`)
	for _, secret := range []string{"private", "secret", "token", "/success", "/failure"} {
		require.NotContains(t, before, secret)
	}
}

func TestRecorder_ConcurrentObservationAndScraping(t *testing.T) {
	m := requestmetrics.New("gateway")
	h := m.HTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
				_ = exposition(m)
			}
		}()
	}
	wg.Wait()
	require.Contains(t, exposition(m), `result="success",status="200"} 2000`)
	require.Contains(t, exposition(m), `method="api",le="+Inf"} 2000`)
}

func TestPanic_IsRecordedAndPropagates(t *testing.T) {
	m := recorder()
	require.PanicsWithValue(t, "synthetic panic", func() {
		_, _ = m.UnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/jobtrack.auth.v1.AuthService/Login"}, func(context.Context, any) (any, error) { panic("synthetic panic") })
	})
	require.Contains(t, exposition(m), `result="error",status="Internal"} 1`)
	gateway := requestmetrics.New("gateway")
	h := gateway.HTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("synthetic panic") }))
	require.Panics(t, func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)) })
	require.Contains(t, exposition(gateway), `result="error",status="500"} 1`)
}
