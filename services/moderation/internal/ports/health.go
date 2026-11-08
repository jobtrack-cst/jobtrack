package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

// HealthChecker is the application boundary for overall service health.
type HealthChecker interface {
	CheckHealth(context.Context) (domain.HealthStatus, error)
}
