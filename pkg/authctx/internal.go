package authctx

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// IsInternalMethod identifies the service component of a gRPC full method name.
func IsInternalMethod(method string) bool {
	parts := strings.Split(method, "/")
	return len(parts) >= 2 && strings.HasSuffix(parts[1], "InternalService")
}

// ValidInternalToken accepts nonempty visible ASCII without whitespace, as required
// for unambiguous gRPC metadata. It never trims or rewrites a secret.
func ValidInternalToken(token string) bool {
	if token == "" {
		return false
	}
	for _, c := range []byte(token) {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

// InternalUnaryServerInterceptor protects internal RPCs using the Section 6
// allowlist. Pass INTERNAL_TOKEN from startup configuration, never request data.
func InternalUnaryServerInterceptor(token string) (grpc.UnaryServerInterceptor, error) {
	if !ValidInternalToken(token) {
		return nil, errors.New("INTERNAL_TOKEN must contain nonempty visible ASCII without whitespace")
	}
	expected := sha256.Sum256([]byte(token))
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !IsInternalMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		tokens := md.Get("x-internal-token")
		if len(tokens) != 1 || !ValidInternalToken(tokens[0]) {
			return nil, status.Error(codes.Unauthenticated, "invalid internal credentials")
		}
		actual := sha256.Sum256([]byte(tokens[0]))
		// Fixed-size digests avoid the length-dependent early return of ConstantTimeCompare.
		if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			return nil, status.Error(codes.Unauthenticated, "invalid internal credentials")
		}
		callers := md.Get("x-caller-service")
		if len(callers) != 1 || !internalCallerAllowed(info.FullMethod, callers[0]) {
			return nil, status.Error(codes.PermissionDenied, "internal caller not allowed")
		}
		return handler(ctx, req)
	}, nil
}

// internalCallerAllowed applies the AGENTS.md Section 6 table. Unknown internal
// methods are denied until their callers are agreed there.
func internalCallerAllowed(method, caller string) bool {
	for _, allowed := range internalCallers(method) {
		if caller == allowed {
			return true
		}
	}
	return false
}

func internalCallers(method string) []string {
	switch method {
	case "/jobtrack.auth.v1.AuthInternalService/GetUser":
		return []string{"notification-dispatcher", "application"}
	case "/jobtrack.notification_inbox.v1.NotificationInboxInternalService/CreateNotification":
		return []string{"notification-dispatcher"}
	case "/jobtrack.auth.v1.AuthInternalService/ListUsersByRole", "/jobtrack.analytics.v1.AnalyticsInternalService/GetInternalSummary":
		return []string{"weekly-summary"}
	case "/jobtrack.category_tag.v1.CategoryTagInternalService/ResolveNames":
		return []string{"job-alert"}
	case "/jobtrack.job_posting.v1.JobPostingInternalService/ListJobsClosingBetween", "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers":
		return []string{"deadline-reminder"}
	case "/jobtrack.resume.v1.ResumeInternalService/GetResumeFile":
		return []string{"resume-validator"}
	default:
		return nil
	}
}
