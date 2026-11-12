package grpc

import (
	"context"
	"errors"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/ports"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// HealthHandler exposes only the overall-service Check endpoint of the reference template.
type HealthHandler struct {
	healthpb.UnimplementedHealthServer
	checker ports.HealthChecker
}

var _ healthpb.HealthServer = (*HealthHandler)(nil)

func NewHealthHandler(checker ports.HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

func (h *HealthHandler) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "health check request is required")
	}
	if req.GetService() != "" {
		return nil, status.Error(codes.NotFound, "unknown service")
	}
	health, err := h.checker.CheckHealth(ctx)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return nil, status.Error(codes.Canceled, "health check canceled")
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, "health check deadline exceeded")
		default:
			return nil, status.Error(codes.Internal, "health check failed")
		}
	}
	var serving healthpb.HealthCheckResponse_ServingStatus
	switch health {
	case domain.Serving:
		serving = healthpb.HealthCheckResponse_SERVING
	case domain.NotServing:
		serving = healthpb.HealthCheckResponse_NOT_SERVING
	case domain.Unknown:
		serving = healthpb.HealthCheckResponse_UNKNOWN
	default:
		return nil, status.Error(codes.Internal, "health check failed")
	}
	return &healthpb.HealthCheckResponse{Status: serving}, nil
}
