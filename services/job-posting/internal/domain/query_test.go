package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name    string
		size    int32
		token   string
		want    domain.Page
		wantErr error
	}{
		{"defaults", 0, "", domain.Page{Limit: 20, Offset: 0}, nil},
		{"size 1", 1, "", domain.Page{Limit: 1}, nil},
		{"size 100", 100, "", domain.Page{Limit: 100}, nil},
		{"size 101 capped", 101, "", domain.Page{Limit: 100}, nil},
		{"size negative", -1, "", domain.Page{}, domain.ErrInvalidPageSize},
		{"token offset", 10, "20", domain.Page{Limit: 10, Offset: 20}, nil},
		{"token zero", 10, "0", domain.Page{Limit: 10, Offset: 0}, nil},
		{"token negative", 10, "-1", domain.Page{}, domain.ErrInvalidPageToken},
		{"token plus sign", 10, "+5", domain.Page{}, domain.ErrInvalidPageToken},
		{"token letters", 10, "abc", domain.Page{}, domain.ErrInvalidPageToken},
		{"token space", 10, " 5", domain.Page{}, domain.ErrInvalidPageToken},
		{"token overflow", 10, "99999999999999999999999", domain.Page{}, domain.ErrInvalidPageToken},
		{"token largest int", 100, "9223372036854775807", domain.Page{Limit: 100, Offset: math.MaxInt}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPage(tt.size, tt.token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPage_NextPageToken(t *testing.T) {
	tests := []struct {
		name  string
		page  domain.Page
		total int
		want  string
	}{
		{"more pages", domain.Page{Limit: 20, Offset: 0}, 45, "20"},
		{"exact last page", domain.Page{Limit: 20, Offset: 20}, 40, ""},
		{"partial last page", domain.Page{Limit: 20, Offset: 40}, 45, ""},
		{"empty", domain.Page{Limit: 20}, 0, ""},
		{"offset past the end", domain.Page{Limit: 20, Offset: 100}, 45, ""},
		{"page ends on the last row", domain.Page{Limit: 20, Offset: 25}, 45, ""},
		{"one row left after the page", domain.Page{Limit: 20, Offset: 24}, 45, "44"},
		{"largest offset does not overflow", domain.Page{Limit: 100, Offset: math.MaxInt}, 5, ""},
		{"near-largest offset does not overflow", domain.Page{Limit: 100, Offset: math.MaxInt - 50}, 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.page.NextPageToken(tt.total))
		})
	}
}

func TestNewClosingWindow(t *testing.T) {
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		wantStart time.Time
		wantEnd   time.Time
		wantErr   error
	}{
		{"48 hours", start, start.Add(48 * time.Hour), start, start.Add(48 * time.Hour), nil},
		{"rounds up to microsecond", start.Add(1), start.Add(time.Hour + 1), start.Add(time.Microsecond), start.Add(time.Hour + time.Microsecond), nil},
		{"missing start", time.Time{}, start, time.Time{}, time.Time{}, domain.ErrInvalidTimeRange},
		{"missing end", start, time.Time{}, time.Time{}, time.Time{}, domain.ErrInvalidTimeRange},
		{"start equals end", start, start, time.Time{}, time.Time{}, domain.ErrInvalidTimeRange},
		{"start after end", start.Add(time.Hour), start, time.Time{}, time.Time{}, domain.ErrInvalidTimeRange},
		{"equal after rounding", start.Add(1), start.Add(2), time.Time{}, time.Time{}, domain.ErrInvalidTimeRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewClosingWindow(tt.start, tt.end)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, got.Start)
			assert.Equal(t, tt.wantEnd, got.End)
		})
	}
}
