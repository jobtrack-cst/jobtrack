package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
)

// Health implements the reference use case without infrastructure dependencies.
type Health struct{ readiness ports.Readiness }

var _ ports.HealthChecker = (*Health)(nil)

func NewHealth() *Health { return &Health{} }

func NewHealthWithReadiness(readiness ports.Readiness) *Health {
	return &Health{readiness: readiness}
}

func (h *Health) CheckHealth(ctx context.Context) (domain.HealthStatus, error) {
	if err := ctx.Err(); err != nil {
		return domain.Unknown, err
	}
	if h.readiness != nil {
		if err := h.readiness.Check(ctx); err != nil {
			return domain.NotServing, err
		}
	}
	return domain.Serving, nil
}
