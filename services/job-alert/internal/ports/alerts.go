package ports

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
)

// AlertRepository serializes creates per applicant, checking duplicates before the limit.
//
//go:generate go tool mockgen -destination=mocks/alerts_mock.go -package=mocks . AlertRepository,NameResolver,Clock
type AlertRepository interface {
	Create(context.Context, domain.Alert) (domain.Alert, error)
	List(context.Context, string, domain.Page) ([]domain.Alert, int64, error)
	Delete(context.Context, string, string) error
}

// NameResolver represents CategoryTagInternalService.ResolveNames; absent IDs are omitted.
type NameResolver interface {
	ResolveNames(context.Context, []string, []string) (domain.Names, error)
}
type Clock interface{ Now() time.Time }
