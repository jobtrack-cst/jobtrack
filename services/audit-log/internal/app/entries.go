package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports"
)

// AuditLog holds the audit-log use cases. Sprint 1 has only the admin list;
// recording entries comes with the Sprint 2 event consumer.
type AuditLog struct {
	repo ports.EntryRepository
}

func NewAuditLog(repo ports.EntryRepository) *AuditLog {
	return &AuditLog{repo: repo}
}

// ListEntriesInput mirrors ListAuditEntriesRequest. A nil time means the
// filter was not set.
type ListEntriesInput struct {
	PageSize  int32
	PageToken string
	EventType string
	Start     *time.Time
	End       *time.Time
}

// EntryPage is one page of results with the shared pagination fields.
type EntryPage struct {
	Entries       []domain.Entry
	NextPageToken string
	TotalCount    int64
}

// ListEntries checks the role before any input, so a non-admin learns
// nothing about which filters are valid.
func (s *AuditLog) ListEntries(ctx context.Context, caller domain.Caller, in ListEntriesInput) (EntryPage, error) {
	if err := caller.RequireAdmin(); err != nil {
		return EntryPage{}, err
	}
	filter, err := domain.NewListFilter(in.EventType, in.Start, in.End)
	if err != nil {
		return EntryPage{}, err
	}
	page, err := domain.NewPage(in.PageSize, in.PageToken)
	if err != nil {
		return EntryPage{}, err
	}
	entries, total, err := s.repo.ListEntries(ctx, filter, page)
	if err != nil {
		return EntryPage{}, fmt.Errorf("list audit entries: %w", err)
	}
	return EntryPage{
		Entries:       entries,
		NextPageToken: page.NextToken(len(entries), total),
		TotalCount:    total,
	}, nil
}
