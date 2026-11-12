package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

func TestNewPage(t *testing.T) {
	for _, tt := range []struct {
		name    string
		size    int32
		token   string
		want    domain.Page
		wantErr error
	}{
		{"zero size means 20", 0, "", domain.Page{Limit: 20}, nil},
		{"size 1", 1, "", domain.Page{Limit: 1}, nil},
		{"size 100", 100, "", domain.Page{Limit: 100}, nil},
		{"size 101 is capped", 101, "", domain.Page{Limit: 100}, nil},
		{"negative size", -1, "", domain.Page{}, domain.ErrInvalidPageSize},
		{"offset token", 0, "20", domain.Page{Limit: 20, Offset: 20}, nil},
		{"zero token", 5, "0", domain.Page{Limit: 5}, nil},
		{"plus sign", 0, "+5", domain.Page{}, domain.ErrInvalidPageToken},
		{"minus sign", 0, "-1", domain.Page{}, domain.ErrInvalidPageToken},
		{"space", 0, " 5", domain.Page{}, domain.ErrInvalidPageToken},
		{"letters", 0, "abc", domain.Page{}, domain.ErrInvalidPageToken},
		{"overflow", 0, "99999999999999999999999", domain.Page{}, domain.ErrInvalidPageToken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPage(tt.size, tt.token)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPage_NextPageToken(t *testing.T) {
	for _, tt := range []struct {
		name  string
		page  domain.Page
		total int
		want  string
	}{
		{"more pages", domain.Page{Limit: 20}, 30, "20"},
		{"exactly one page", domain.Page{Limit: 20}, 20, ""},
		{"last partial page", domain.Page{Limit: 20, Offset: 20}, 30, ""},
		{"empty list", domain.Page{Limit: 20}, 0, ""},
		{"offset past the end", domain.Page{Limit: 20, Offset: 50}, 30, ""},
		{"one row left", domain.Page{Limit: 10, Offset: 10}, 21, "20"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.page.NextPageToken(tt.total))
		})
	}
}
