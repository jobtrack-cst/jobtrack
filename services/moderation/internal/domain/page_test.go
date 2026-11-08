package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name     string
		pageSize int32
		token    string
		want     domain.Page
		wantErr  error
	}{
		{"zero size uses default", 0, "", domain.Page{Limit: 20, Offset: 0}, nil},
		{"size 1", 1, "", domain.Page{Limit: 1, Offset: 0}, nil},
		{"size 100 is kept", 100, "", domain.Page{Limit: 100, Offset: 0}, nil},
		{"size 101 is capped", 101, "", domain.Page{Limit: 100, Offset: 0}, nil},
		{"negative size", -1, "", domain.Page{}, domain.ErrInvalidPageSize},
		{"offset token", 20, "20", domain.Page{Limit: 20, Offset: 20}, nil},
		{"zero token", 20, "0", domain.Page{Limit: 20, Offset: 0}, nil},
		{"plus sign token", 20, "+5", domain.Page{}, domain.ErrInvalidPageToken},
		{"leading zero token", 20, "007", domain.Page{}, domain.ErrInvalidPageToken},
		{"negative token", 20, "-1", domain.Page{}, domain.ErrInvalidPageToken},
		{"text token", 20, "abc", domain.Page{}, domain.ErrInvalidPageToken},
		{"spaced token", 20, " 20", domain.Page{}, domain.ErrInvalidPageToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPage(tt.pageSize, tt.token)
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
		{"more pages", domain.Page{Limit: 20, Offset: 0}, 20, 45, "20"},
		{"exactly the last page", domain.Page{Limit: 20, Offset: 20}, 20, 40, ""},
		{"short last page", domain.Page{Limit: 20, Offset: 40}, 5, 45, ""},
		{"empty result", domain.Page{Limit: 20, Offset: 60}, 0, 45, ""},
		{"no rows at all", domain.Page{Limit: 20, Offset: 0}, 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.page.NextToken(tt.returned, tt.total))
		})
	}
}
