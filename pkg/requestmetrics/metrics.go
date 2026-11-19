// Package requestmetrics records bounded application request metrics without a
// global registry. Each server instance owns a Recorder shared with /metrics.
package requestmetrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Buckets are seconds, with an implicit +Inf bucket in the exposition.
var bounds = [...]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

type resultKey struct{ method, result, status string }
type duration struct {
	Count   uint64
	Sum     float64
	Buckets [len(bounds)]uint64
}

// Recorder uses one short critical section per completed request. Scraping
// copies a snapshot before writing so a slow metrics client cannot block calls.
type Recorder struct {
	mu        sync.Mutex
	service   string
	requests  map[resultKey]uint64
	durations map[string]duration
}

// New's service must be a fixed application name, never request/configuration data.
func New(service string) *Recorder {
	return &Recorder{service: service, requests: make(map[resultKey]uint64), durations: make(map[string]duration)}
}

// RegisterGRPC seeds zero series from server registration, never client input.
// Call after registering handlers and before serving requests.
func (m *Recorder) RegisterGRPC(server *grpc.Server) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for service, info := range server.GetServiceInfo() {
		if service == "grpc.health.v1.Health" {
			continue
		}
		for _, method := range info.Methods {
			if method.IsClientStream || method.IsServerStream {
				continue
			}
			name := "/" + service + "/" + method.Name
			m.durations[name] = duration{}
			m.requests[resultKey{name, "success", "OK"}] = 0
		}
	}
}

func (m *Recorder) observe(method, result, code string, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[resultKey{method, result, code}]++
	d := m.durations[method]
	d.Count++
	seconds := elapsed.Seconds()
	d.Sum += seconds
	for i, upper := range bounds {
		if seconds <= upper {
			d.Buckets[i]++
		}
	}
	m.durations[method] = d
}

// UnaryServerInterceptor must precede authentication in the interceptor chain
// so denied calls are included. It never changes responses, errors or metadata.
func (m *Recorder) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		m.mu.Lock()
		_, known := m.durations[info.FullMethod]
		m.mu.Unlock()
		method := "unknown"
		if known {
			method = info.FullMethod
		}
		start := time.Now()
		completed := false
		defer func() {
			code := codes.Internal
			if completed {
				rpcStatus, ok := status.FromError(err)
				if !ok {
					rpcStatus = status.FromContextError(err)
				}
				code = rpcStatus.Code()
			}
			if code > codes.Unauthenticated {
				code = codes.Unknown
			}
			result := "success"
			if code != codes.OK {
				result = "error"
			}
			m.observe(method, result, code.String(), time.Since(start))
		}()
		response, err = handler(ctx, req)
		completed = true
		return response, err
	}
}

// HTTP measures the entire gateway handler, including authentication failures.
// The fixed api label aggregates all paths/methods, including unmatched routes.
// httpsnoop preserves optional ResponseWriter interfaces and first final status.
func (m *Recorder) HTTP(next http.Handler) http.Handler {
	m.mu.Lock()
	m.durations["api"] = duration{}
	m.requests[resultKey{"api", "success", "200"}] = 0
	m.mu.Unlock()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && (r.URL.Path == "/metrics" || r.URL.Path == "/healthz") {
			next.ServeHTTP(w, r)
			return
		}
		metrics := httpsnoop.Metrics{Code: http.StatusOK}
		completed := false
		defer func() {
			code := metrics.Code
			if !completed {
				code = http.StatusInternalServerError
			}
			codeLabel := "other"
			if code >= 100 && code <= 599 {
				codeLabel = strconv.Itoa(code)
			}
			result := "success"
			if code >= 400 {
				result = "error"
			}
			m.observe("api", result, codeLabel, metrics.Duration)
		}()
		metrics.CaptureMetrics(w, func(wrapped http.ResponseWriter) { next.ServeHTTP(wrapped, r) })
		completed = true
	})
}

// WritePrometheus appends Prometheus text exposition to the existing readiness
// response. The caller owns HTTP headers. No request data or error text is stored.
func (m *Recorder) WritePrometheus(w io.Writer) {
	m.mu.Lock()
	requests := make(map[resultKey]uint64, len(m.requests))
	durations := make(map[string]duration, len(m.durations))
	for k, v := range m.requests {
		requests[k] = v
	}
	for k, v := range m.durations {
		durations[k] = v
	}
	m.mu.Unlock()
	keys := make([]resultKey, 0, len(requests))
	for k := range requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.method != b.method {
			return a.method < b.method
		}
		if a.result != b.result {
			return a.result < b.result
		}
		return a.status < b.status
	})
	fmt.Fprint(w, "# HELP jobtrack_requests_total Completed application requests by result and status.\n# TYPE jobtrack_requests_total counter\n")
	for _, k := range keys {
		fmt.Fprintf(w, "jobtrack_requests_total{service=%q,method=%q,result=%q,status=%q} %d\n", m.service, k.method, k.result, k.status, requests[k])
	}
	fmt.Fprint(w, "# HELP jobtrack_request_duration_seconds Application request duration in seconds.\n# TYPE jobtrack_request_duration_seconds histogram\n")
	methods := make([]string, 0, len(durations))
	for method := range durations {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		d := durations[method]
		labels := fmt.Sprintf("service=%q,method=%q", m.service, method)
		for i, upper := range bounds {
			fmt.Fprintf(w, "jobtrack_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, strconv.FormatFloat(upper, 'g', -1, 64), d.Buckets[i])
		}
		fmt.Fprintf(w, "jobtrack_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, d.Count)
		fmt.Fprintf(w, "jobtrack_request_duration_seconds_sum{%s} %g\n", labels, d.Sum)
		fmt.Fprintf(w, "jobtrack_request_duration_seconds_count{%s} %d\n", labels, d.Count)
	}
}
