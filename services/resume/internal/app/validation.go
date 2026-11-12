package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// ValidationResults processes typed results; parsing and settlement belong to transport.
type ValidationResults struct {
	repo  ports.ValidationRepository
	clock ports.Clock
}

func NewValidationResults(repo ports.ValidationRepository, clock ports.Clock) *ValidationResults {
	return &ValidationResults{repo: repo, clock: clock}
}
func (s *ValidationResults) Apply(ctx context.Context, input domain.ValidationResult) (domain.ValidationOutcome, error) {
	result, err := domain.NewValidationResult(input.EventID, input.ResumeID, input.ApplicantID, input.Status, input.Reason)
	if err != nil {
		return domain.ValidationOutcomeUnspecified, err
	}
	return s.repo.ApplyValidation(ctx, result, s.clock.Now().UTC())
}
