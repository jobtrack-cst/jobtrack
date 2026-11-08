package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/ports"
)

// Analytics holds the analytics use cases. Sprint 1 has only the two
// summaries; recording events comes with the Sprint 2 event consumer.
type Analytics struct {
	repo ports.OutcomeRepository
}

func NewAnalytics(repo ports.OutcomeRepository) *Analytics {
	return &Analytics{repo: repo}
}

// SummaryInput mirrors GetSummaryRequest and GetInternalSummaryRequest. A
// nil time means the field was not set.
type SummaryInput struct {
	Start *time.Time
	End   *time.Time
}

// GetSummary checks the role before any input, so a non-admin learns
// nothing about which time ranges are valid.
func (a *Analytics) GetSummary(ctx context.Context, caller domain.Caller, in SummaryInput) (domain.Summary, error) {
	if err := caller.RequireAdmin(); err != nil {
		return domain.Summary{}, err
	}
	return a.summarize(ctx, in)
}

// GetInternalSummary has no user caller. The internal server interceptor has
// already checked INTERNAL_TOKEN and that the caller is weekly-summary.
func (a *Analytics) GetInternalSummary(ctx context.Context, in SummaryInput) (domain.Summary, error) {
	return a.summarize(ctx, in)
}

// summarize is shared, so both RPCs return the same counts for the same
// range, as the contract says.
func (a *Analytics) summarize(ctx context.Context, in SummaryInput) (domain.Summary, error) {
	r, err := domain.NewTimeRange(in.Start, in.End)
	if err != nil {
		return domain.Summary{}, err
	}
	counts, err := a.repo.CountEvents(ctx, r)
	if err != nil {
		return domain.Summary{}, fmt.Errorf("count outcome events: %w", err)
	}
	return domain.BuildSummary(counts), nil
}
