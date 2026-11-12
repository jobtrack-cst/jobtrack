package ports

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

// ApplicationEventRepository commits input deduplication, status, history and outbox together.
type ApplicationEventRepository interface {
	ApplyEvent(context.Context, domain.ApplicationEvent) error
}
