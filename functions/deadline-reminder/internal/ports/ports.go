// Package ports defines what deadline-reminder needs from other services.
package ports

import (
	"context"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/ports_mock.go -package=mocks . Jobs,Savers,Publisher

// Jobs reads published jobs closing in [start, end) one page at a time
// (JobPostingInternalService.ListJobsClosingBetween). next is empty on the last page.
type Jobs interface {
	ListClosing(ctx context.Context, start, end time.Time, pageToken string) (jobs []domain.ClosingJob, next string, err error)
}

// Savers reads the applicants who saved one job, one page at a time
// (SavedJobsInternalService.ListJobSavers). next is empty on the last page.
type Savers interface {
	ListSavers(ctx context.Context, jobID, pageToken string) (applicantIDs []string, next string, err error)
}

// Publisher publishes one event envelope through pkg/events.
type Publisher interface {
	Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error
}
