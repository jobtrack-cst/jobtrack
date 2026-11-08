package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

// Internal test: the WHERE clause uses only bind parameters, numbered in the
// order the filters are added, with start inclusive and end exclusive.
func TestEntryWhere(t *testing.T) {
	start := time.Date(2026, 11, 8, 4, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, tt := range []struct {
		name      string
		filter    domain.ListFilter
		wantWhere string
		wantArgs  []any
	}{
		{"no filters", domain.ListFilter{}, "", nil},
		{"event type", domain.ListFilter{EventType: "job.published"}, " WHERE event_type = $1", []any{"job.published"}},
		{"start only", domain.ListFilter{Start: &start}, " WHERE occurred_at >= $1", []any{start}},
		{"end only", domain.ListFilter{End: &end}, " WHERE occurred_at < $1", []any{end}},
		{
			"all filters",
			domain.ListFilter{EventType: "offer.made", Start: &start, End: &end},
			" WHERE event_type = $1 AND occurred_at >= $2 AND occurred_at < $3",
			[]any{"offer.made", start, end},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			where, args := entryWhere(tt.filter)
			if where != tt.wantWhere {
				t.Fatalf("where=%q, want %q", where, tt.wantWhere)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args=%v, want %v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("arg %d=%v, want %v", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestSafe_HidesDetailsButKeepsCancellation(t *testing.T) {
	err := safe(context.Background(), "list audit entries failed")
	if err == nil || err.Error() != "list audit entries failed" {
		t.Fatalf("err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := safe(ctx, "list audit entries failed"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
