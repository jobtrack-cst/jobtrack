package grpc

import (
	"context"
	"errors"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

// CloseJob is only called by the job.closed consumer, never by the gRPC handlers.
func (*fakeRepo) CloseJob(context.Context, domain.JobClosure) (domain.ClosureOutcome, error) {
	return domain.ClosureOutcome{}, errors.New("CloseJob is not used by the gRPC handlers")
}
