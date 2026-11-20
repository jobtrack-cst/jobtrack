package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// InternalError retains a cause until the adapter logs it. Message must be a
// fixed, code-owned diagnostic: never interpolate input or another error's text.
// It emits no log and exposes only message through Error and GRPCStatus.
func InternalError(message string, cause error) error {
	return &internalError{message: message, cause: cause}
}

type internalError struct {
	message string
	cause   error
}

func (e *internalError) Error() string              { return e.message }
func (e *internalError) Unwrap() error              { return e.cause }
func (e *internalError) GRPCStatus() *status.Status { return status.New(codes.Internal, e.message) }

// GRPCError is called once at the adapter's error return, with the original
// cause and its mapped status. Only INTERNAL emits a log. The returned status
// has no cause or details. Method and mapped INTERNAL message must be code-owned
// constants; logger must come from New and contain no private attributes.
func GRPCError(ctx context.Context, logger *slog.Logger, method string, cause, mapped error) error {
	if status.Code(mapped) != codes.Internal {
		return mapped
	}
	attrs := []any{"grpc_method", method, "code", codes.Internal.String(), "cause", diagnostic(cause)}
	// Incoming metadata is untrusted. Accept only a single canonical UUID for
	// correlation; never copy arbitrary header text or other metadata into logs.
	md, _ := metadata.FromIncomingContext(ctx)
	if ids := md.Get("x-request-id"); len(ids) == 1 {
		if id, err := uuid.Parse(ids[0]); err == nil && id != uuid.Nil && id.String() == ids[0] {
			attrs = append(attrs, "request_id", ids[0])
		}
	}
	logger.ErrorContext(ctx, "unexpected gRPC error", attrs...)
	return status.Error(codes.Internal, status.Convert(mapped).Message())
}

// No arbitrary Error(), String(), LogValue() or gRPC message is rendered. Safe
// wrappers retain static operations; other errors contribute only type and code.
// Bound traversal so a pathological error chain cannot flood logs or loop.
func diagnostic(err error) string {
	parts := make([]string, 0, 8)
	for i := 0; err != nil && i < 8; i++ {
		if safe, ok := err.(*internalError); ok {
			parts = append(parts, safe.message)
		} else {
			parts = append(parts, fmt.Sprintf("%T (code=%s)", err, diagnosticCode(err)))
		}
		next, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = next.Unwrap()
	}
	if len(parts) == 0 {
		return "unexpected failure (details withheld)"
	}
	return strings.Join(parts, ": ")
}

func diagnosticCode(err error) codes.Code {
	// status.Code calls Error() for non-status errors and wrapped statuses.
	// Inspect only the current node; traversal handles wrappers separately.
	if s, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
		if st := s.GRPCStatus(); st != nil {
			return st.Code()
		}
	}
	return codes.Unknown
}
