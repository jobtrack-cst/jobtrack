package app

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/ports"
)

// Clock supplies the current time so tests can control the open-jobs cutoff.
type Clock interface {
	Now() time.Time
}

// Search validates filters and reads open jobs from the read model.
type Search struct {
	repo  ports.SearchRepository
	clock Clock
}

func NewSearch(repo ports.SearchRepository, clock Clock) *Search {
	return &Search{repo: repo, clock: clock}
}

// Result is what the handler needs to build SearchJobsResponse.
type Result struct {
	Jobs  []domain.JobSummary
	Total int
}

// Run validates raw inputs, keeps only open jobs, and returns the page.
func (s *Search) Run(ctx context.Context, in RawInput) (Result, error) {
	filters, err := domain.NewFilters(in.Query, in.CategoryID, in.TagIDs, in.Location, in.PageSize, in.PageToken, domain.IsUUID)
	if err != nil {
		return Result{}, err
	}
	jobs, total, err := s.repo.Search(ctx, filters)
	if err != nil {
		return Result{}, err
	}
	now := s.clock.Now()
	open := jobs[:0]
	for _, j := range jobs {
		if j.IsOpen(now) {
			open = append(open, j)
		}
	}
	return Result{Jobs: open, Total: total}, nil
}

// RawInput mirrors the proto request fields before validation.
type RawInput struct {
	Query      string
	CategoryID string
	TagIDs     []string
	Location   string
	PageSize   int
	PageToken  string
}
