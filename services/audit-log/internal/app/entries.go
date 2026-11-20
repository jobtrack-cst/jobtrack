package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports"
)

// ErrInvalidEntry marks an event that fails the JT-35 field limits. The
// consumer dead-letters it, because a retry can never make it valid.
var ErrInvalidEntry = errors.New("event cannot be recorded as an audit entry")

// AuditLog holds the audit-log use cases: the admin list and recording
// entries from the event consumer (JT-72).
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

// RecordEntryInput is one consumed event envelope.
type RecordEntryInput struct {
	EventID    string
	EventType  string
	Producer   string
	OccurredAt time.Time
	DataJSON   string
}

// RecordEntry validates the event with the JT-35 rules and stores it once.
// recorded is false, with no error, when the event_id was already processed.
func (s *AuditLog) RecordEntry(ctx context.Context, in RecordEntryInput) (bool, error) {
	entry, err := domain.NewEntry(in.EventID, in.EventType, in.Producer, in.OccurredAt, in.DataJSON)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidEntry, err)
	}
	recorded, err := s.repo.RecordEntry(ctx, entry)
	if err != nil {
		return false, fmt.Errorf("record audit entry: %w", err)
	}
	return recorded, nil
}
