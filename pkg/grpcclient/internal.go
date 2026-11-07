package grpcclient

import (
	"context"
	"errors"

	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// InternalUnaryClientInterceptor includes the shared timeout, retry and circuit
// breaker policy. Use instead of UnaryClientInterceptor for internal callers.
// Values must come from INTERNAL_TOKEN and SERVICE_NAME at startup.
func InternalUnaryClientInterceptor(token, caller string) (grpc.UnaryClientInterceptor, error) {
	if !authctx.ValidInternalToken(token) {
		return nil, errors.New("invalid INTERNAL_TOKEN configuration")
	}
	switch caller {
	case "job-alert", "notification-dispatcher", "weekly-summary", "deadline-reminder", "resume-validator":
	default:
		return nil, errors.New("SERVICE_NAME is not an approved internal caller")
	}
	shared := UnaryClientInterceptor()
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Remove pre-existing security metadata on both public and internal calls.
		// Only configured credentials may be sent; incoming metadata is never copied.
		md, _ := metadata.FromOutgoingContext(ctx)
		if md == nil {
			md = metadata.MD{}
		}
		md.Delete("x-internal-token")
		md.Delete("x-caller-service")
		if authctx.IsInternalMethod(method) {
			md.Set("x-internal-token", token)
			md.Set("x-caller-service", caller)
		}
		return shared(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, invoker, opts...)
	}, nil
}
