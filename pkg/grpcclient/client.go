package grpcclient

import (
	"context"
	"errors"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	requestTimeout = 2 * time.Second
	maxRetries     = 3
	retryDelay     = 50 * time.Millisecond
)

// UnaryClientInterceptor applies a shared timeout, retries, and circuit protection.
// Create one per downstream connection; install with grpc.WithChainUnaryInterceptor.
// The circuit opens after five consecutive unavailable calls and probes after 30s.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	breaker := gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Timeout: 30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			// Caller and business errors do not indicate downstream unavailability.
			return status.Code(err) != codes.Unavailable
		},
	})
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		_, err := breaker.Execute(func() (struct{}, error) {
			return struct{}{}, invoke(ctx, method, req, reply, cc, invoker, opts...)
		})
		if errors.Is(err, gobreaker.ErrOpenState) {
			return status.Error(codes.Unavailable, "downstream circuit breaker is open")
		}
		if errors.Is(err, gobreaker.ErrTooManyRequests) {
			return status.Error(codes.Unavailable, "downstream circuit breaker probe is in progress")
		}
		return err
	}
}

func invoke(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		if ctx.Err() != nil {
			return status.FromContextError(ctx.Err()).Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return status.FromContextError(err).Err()
		}
		if status.Code(err) != codes.Unavailable || attempt == maxRetries {
			return err
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return status.FromContextError(ctx.Err()).Err()
		case <-timer.C:
		}
	}
}
