package ports

import (
	"context"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/jobs_mock.go -package=mocks . JobRepository,JobTransaction

// JobRepository commits all callback operations together, or rolls them all back.
type JobRepository interface {
	WithinTransaction(context.Context, func(JobTransaction) error) error
}
type JobTransaction interface {
	Claim(context.Context, string) (bool, error)
	// Candidates returns a single subscription snapshot, narrowed by SQL filters.
	Candidates(context.Context, domain.PublishedJob) ([]domain.Alert, error)
	SaveMatches(context.Context, string, string, []domain.Match) error
	AddEvent(context.Context, *eventsv1.EventEnvelope) error
}
