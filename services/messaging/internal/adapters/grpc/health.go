package grpc

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// HealthHandler exposes only the overall-service Check endpoint of the reference template.
type HealthHandler struct {
	healthpb.UnimplementedHealthServer
	checker ports.HealthChecker
	logger  *slog.Logger
}

var _ healthpb.HealthServer = (*HealthHandler)(nil)

func NewHealthHandler(checker ports.HealthChecker, loggers ...*slog.Logger) *HealthHandler {
	logger := logging.New("messaging", os.Stdout)
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &HealthHandler{checker: checker, logger: logger}
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
			return nil, logging.GRPCError(ctx, h.logger, healthpb.Health_Check_FullMethodName, err, status.Error(codes.Internal, "health check failed"))
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
		return nil, logging.GRPCError(ctx, h.logger, healthpb.Health_Check_FullMethodName, logging.InternalError("invalid health status", nil), status.Error(codes.Internal, "health check failed"))
	}
	return &healthpb.HealthCheckResponse{Status: serving}, nil
}
