package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name    string
		size    int32
		token   string
		want    domain.Page
		wantErr error
	}{
		{"zero size uses default", 0, "", domain.Page{Limit: 20, Offset: 0}, nil},
		{"size 1", 1, "", domain.Page{Limit: 1, Offset: 0}, nil},
		{"size 100 is kept", 100, "", domain.Page{Limit: 100, Offset: 0}, nil},
		{"size 101 is capped", 101, "", domain.Page{Limit: 100, Offset: 0}, nil},
		{"size -1 is invalid", -1, "", domain.Page{}, domain.ErrInvalidPageSize},
		{"token 0", 10, "0", domain.Page{Limit: 10, Offset: 0}, nil},
		{"token 20", 10, "20", domain.Page{Limit: 10, Offset: 20}, nil},
		{"negative token", 10, "-1", domain.Page{}, domain.ErrInvalidPageToken},
		{"plus sign token", 10, "+5", domain.Page{}, domain.ErrInvalidPageToken},
		{"leading zero token", 10, "007", domain.Page{}, domain.ErrInvalidPageToken},
		{"text token", 10, "abc", domain.Page{}, domain.ErrInvalidPageToken},
		{"padded token", 10, " 5", domain.Page{}, domain.ErrInvalidPageToken},
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

func TestPage_NextToken(t *testing.T) {
	tests := []struct {
		name     string
		page     domain.Page
		returned int
		total    int64
		want     string
	}{
		{"more pages", domain.Page{Limit: 2, Offset: 0}, 2, 5, "2"},
		{"middle page", domain.Page{Limit: 2, Offset: 2}, 2, 5, "4"},
		{"last partial page", domain.Page{Limit: 2, Offset: 4}, 1, 5, ""},
		{"last full page", domain.Page{Limit: 2, Offset: 3}, 2, 5, ""},
		{"empty page", domain.Page{Limit: 2, Offset: 10}, 0, 5, ""},
		{"no entries", domain.Page{Limit: 20, Offset: 0}, 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.page.NextToken(tt.returned, tt.total))
		})
	}
}
