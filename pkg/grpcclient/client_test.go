package grpcclient_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryClientInterceptor_RetriesOnlyUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		code     codes.Code
		calls    int
		wantCode codes.Code
	}{
		{"first attempt succeeds", 0, codes.Unavailable, 1, codes.OK},
		{"recovers on second attempt", 1, codes.Unavailable, 2, codes.OK},
		{"recovers on fourth attempt", 3, codes.Unavailable, 4, codes.OK},
		{"stops after three retries", 4, codes.Unavailable, 4, codes.Unavailable},
		{"invalid argument", 4, codes.InvalidArgument, 1, codes.InvalidArgument},
		{"permission denied", 4, codes.PermissionDenied, 1, codes.PermissionDenied},
		{"unauthenticated", 4, codes.Unauthenticated, 1, codes.Unauthenticated},
		{"not found", 4, codes.NotFound, 1, codes.NotFound},
		{"failed precondition", 4, codes.FailedPrecondition, 1, codes.FailedPrecondition},
		{"internal", 4, codes.Internal, 1, codes.Internal},
		{"resource exhausted", 4, codes.ResourceExhausted, 1, codes.ResourceExhausted},
		{"unknown", 4, codes.Unknown, 1, codes.Unknown},
		{"already exists", 4, codes.AlreadyExists, 1, codes.AlreadyExists},
		{"aborted", 4, codes.Aborted, 1, codes.Aborted},
		{"out of range", 4, codes.OutOfRange, 1, codes.OutOfRange},
		{"unimplemented", 4, codes.Unimplemented, 1, codes.Unimplemented},
		{"data loss", 4, codes.DataLoss, 1, codes.DataLoss},
		{"server canceled", 4, codes.Canceled, 1, codes.Canceled},
		{"server deadline", 4, codes.DeadlineExceeded, 1, codes.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := grpcclient.UnaryClientInterceptor()(context.Background(), "/test/Call", nil, nil, nil,
				func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
					calls++
					if calls <= tt.failures {
						return status.Error(tt.code, "original error")
					}
					return nil
				})
			if calls != tt.calls || status.Code(err) != tt.wantCode {
				t.Fatalf("calls=%d code=%v; want calls=%d code=%v", calls, status.Code(err), tt.calls, tt.wantCode)
			}
			if err != nil && status.Convert(err).Message() != "original error" {
				t.Errorf("original status message lost: %v", err)
			}
		})
	}
}

func TestUnaryClientInterceptor_PreservesContextMetadataAndCallArguments(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "value")
	incoming := metadata.Pairs("incoming-only", "unchanged")
	outgoing := metadata.Pairs("request-id", "request-123", "authorization", "synthetic-token")
	ctx = metadata.NewIncomingContext(ctx, incoming)
	ctx = metadata.NewOutgoingContext(ctx, outgoing)
	req, reply := new(int), new(int)
	conn := new(grpc.ClientConn)
	option := grpc.WaitForReady(true)
	var firstDeadline time.Time
	calls := 0
	err := grpcclient.UnaryClientInterceptor()(ctx, "/test/Call", req, reply, conn,
		func(callCtx context.Context, method string, gotReq, gotReply any, gotConn *grpc.ClientConn, opts ...grpc.CallOption) error {
			calls++
			if callCtx.Value(contextKey{}) != "value" || method != "/test/Call" || gotReq != req || gotReply != reply || gotConn != conn || !reflect.DeepEqual(opts, []grpc.CallOption{option}) {
				t.Error("context value or call arguments changed")
			}
			gotIncoming, _ := metadata.FromIncomingContext(callCtx)
			gotOutgoing, _ := metadata.FromOutgoingContext(callCtx)
			if !reflect.DeepEqual(gotIncoming, incoming) || !reflect.DeepEqual(gotOutgoing, outgoing) {
				t.Error("metadata changed or incoming metadata was forwarded implicitly")
			}
			deadline, ok := callCtx.Deadline()
			if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 2*time.Second {
				t.Error("expected a default deadline within two seconds")
			}
			if calls == 1 {
				firstDeadline = deadline
				if time.Until(deadline) < time.Second {
					t.Error("default timeout is shorter than two seconds")
				}
				return status.Error(codes.Unavailable, "retry")
			}
			if !deadline.Equal(firstDeadline) {
				t.Error("retry reset the total request deadline")
			}
			return nil
		}, option)
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_RejectsFinishedCallerContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		code codes.Code
	}{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}, codes.Canceled},
		{"expired deadline", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}, codes.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.ctx()
			defer cancel()
			calls := 0
			err := grpcclient.UnaryClientInterceptor()(ctx, "/test/Call", nil, nil, nil,
				func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
					calls++
					return nil
				})
			if calls != 0 || status.Code(err) != tt.code {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestUnaryClientInterceptor_RespectsCallerDeadlineDuringCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	calls := 0
	err := grpcclient.UnaryClientInterceptor()(ctx, "/test/Call", nil, nil, nil,
		func(callCtx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			calls++
			if got, _ := callCtx.Deadline(); !got.Equal(deadline) {
				t.Error("caller deadline was extended")
			}
			<-callCtx.Done()
			return callCtx.Err()
		})
	if calls != 1 || status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_DefaultTimeoutCancelsCall(t *testing.T) {
	calls := 0
	err := grpcclient.UnaryClientInterceptor()(context.Background(), "/test/Call", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			calls++
			<-ctx.Done()
			return ctx.Err()
		})
	if calls != 1 || status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_CancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	err := grpcclient.UnaryClientInterceptor()(ctx, "/test/Call", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			cancel()
			return status.Error(codes.Unavailable, "retry")
		})
	if calls != 1 || status.Code(err) != codes.Canceled {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_DeadlineStopsRetryWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	calls := 0
	err := grpcclient.UnaryClientInterceptor()(ctx, "/test/Call", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "retry")
		})
	if calls != 1 || status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_OpensCircuitAfterRepeatedFailures(t *testing.T) {
	interceptor := grpcclient.UnaryClientInterceptor()
	calls := 0
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		return status.Error(codes.Unavailable, "downstream unavailable")
	}
	for range 5 {
		if err := interceptor(context.Background(), "/test/Call", nil, nil, nil, invoker); status.Code(err) != codes.Unavailable {
			t.Fatal(err)
		}
	}
	if calls != 20 {
		t.Fatalf("got %d attempts before circuit opens, want 20", calls)
	}
	err := interceptor(context.Background(), "/test/Call", nil, nil, nil, invoker)
	if calls != 20 || status.Code(err) != codes.Unavailable || !strings.Contains(status.Convert(err).Message(), "circuit breaker") {
		t.Fatalf("open circuit did not protect outgoing calls: calls=%d err=%v", calls, err)
	}
}

func TestUnaryClientInterceptor_BusinessErrorsDoNotOpenCircuit(t *testing.T) {
	interceptor := grpcclient.UnaryClientInterceptor()
	calls := 0
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		return status.Error(codes.PermissionDenied, "not allowed")
	}
	for range 10 {
		if err := interceptor(context.Background(), "/test/Call", nil, nil, nil, invoker); status.Code(err) != codes.PermissionDenied {
			t.Fatal(err)
		}
	}
	if calls != 10 {
		t.Fatalf("business errors opened circuit or were retried: calls=%d", calls)
	}
}
