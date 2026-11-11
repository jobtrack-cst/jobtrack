// Package ports defines the boundaries between the domain/application and the
// outside world. Adapters (Postgres, gRPC, RabbitMQ) implement these.
package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/search_mock.go -package=mocks . SearchRepository

// SearchRepository reads the job read model. It returns only open jobs
// (closed = false and deadline after now), already sorted by deadline
// soonest first with job_id breaking ties. The second return is the total
// number of matching open jobs, for pagination.
type SearchRepository interface {
	Search(ctx context.Context, filters domain.Filters) (jobs []domain.JobSummary, total int, err error)
}