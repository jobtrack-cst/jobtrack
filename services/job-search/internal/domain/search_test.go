package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

var uuid = func(s string) bool {
	// 8-4-4-4-12 hex, enough for tests.
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

const (
	validUUID1 = "11111111-1111-1111-1111-111111111111"
	validUUID2 = "22222222-2222-2222-2222-222222222222"
)

func TestIsOpen(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		deadline time.Time
		want     bool
	}{
		{"deadline after now", now.Add(time.Hour), true},
		{"deadline equal now", now, false},
		{"deadline before now", now.Add(-time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := domain.JobSummary{Deadline: tt.deadline}
			if got := j.IsOpen(now); got != tt.want {
				t.Fatalf("IsOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFilters(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		categoryID string
		tagIDs     []string
		location   string
		pageSize   int
		pageToken  string
		want       domain.Filters
		wantErr    error
	}{
		{
			name: "defaults",
			want: domain.Filters{PageSize: domain.DefaultPageSize},
		},
		{
			name:  "query normalized",
			query: "  Hello World  ",
			want:  domain.Filters{Query: "hello world", PageSize: domain.DefaultPageSize},
		},
		{
			name:       "valid category",
			categoryID: validUUID1,
			want:       domain.Filters{CategoryID: validUUID1, PageSize: domain.DefaultPageSize},
		},
		{
			name:       "invalid category",
			categoryID: "not-a-uuid",
			wantErr:    domain.ErrInvalidCategoryID,
		},
		{
			name:   "tags deduped",
			tagIDs: []string{validUUID1, validUUID1, validUUID2},
			want:   domain.Filters{TagIDs: []string{validUUID1, validUUID2}, PageSize: domain.DefaultPageSize},
		},
		{
			name:    "invalid tag",
			tagIDs:  []string{"nope"},
			wantErr: domain.ErrInvalidTagID,
		},
		{
			name:     "valid location",
			location: "  Thimphu ",
			want:     domain.Filters{Location: "Thimphu", PageSize: domain.DefaultPageSize},
		},
		{
			name:     "location too long",
			location: string(make([]rune, 101)),
			wantErr:  domain.ErrInvalidLocation,
		},
		{
			name:     "page size zero uses default",
			pageSize: 0,
			want:     domain.Filters{PageSize: domain.DefaultPageSize},
		},
		{
			name:     "page size too large",
			pageSize: 101,
			wantErr:  domain.ErrInvalidPageSize,
		},
		{
			name:      "page token offset",
			pageToken: "40",
			want:      domain.Filters{PageSize: domain.DefaultPageSize, PageOffset: 40},
		},
		{
			name:      "page token invalid",
			pageToken: "abc",
			wantErr:   domain.ErrInvalidPageToken,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewFilters(tt.query, tt.categoryID, tt.tagIDs, tt.location, tt.pageSize, tt.pageToken, uuid)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Query != tt.want.Query || got.CategoryID != tt.want.CategoryID || got.Location != tt.want.Location || got.PageSize != tt.want.PageSize || got.PageOffset != tt.want.PageOffset {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			if len(got.TagIDs) != len(tt.want.TagIDs) {
				t.Fatalf("TagIDs len = %d, want %d", len(got.TagIDs), len(tt.want.TagIDs))
			}
			for i := range got.TagIDs {
				if got.TagIDs[i] != tt.want.TagIDs[i] {
					t.Fatalf("TagIDs[%d] = %q, want %q", i, got.TagIDs[i], tt.want.TagIDs[i])
				}
			}
		})
	}
}
