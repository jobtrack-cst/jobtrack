package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/entries_mock.go -package=mocks . EntryRepository

// EntryRepository stores audit entries and the processed event IDs that make
// the Sprint 2 consumer idempotent.
type EntryRepository interface {
	// RecordEntry marks the event_id processed and stores the entry in one
	// transaction. recorded is false, with no error, when the event_id was
	// already processed, so the consumer can acknowledge the duplicate.
	RecordEntry(ctx context.Context, entry domain.Entry) (recorded bool, err error)
	// ListEntries returns one page, newest first by occurred_at and then
	// event_id, and the total number of entries matching the filter.
	ListEntries(ctx context.Context, filter domain.ListFilter, page domain.Page) ([]domain.Entry, int64, error)
}
