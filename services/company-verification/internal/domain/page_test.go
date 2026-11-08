package domain_test

import (
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

func TestNewPage(t *testing.T) {
	for _, tt := range []struct {
		name    string
		size    int32
		token   string
		want    domain.Page
		wantErr error
	}{
		{"zero size uses default", 0, "", domain.Page{Limit: 20}, nil},
		{"size 1", 1, "", domain.Page{Limit: 1}, nil},
		{"size 100", 100, "", domain.Page{Limit: 100}, nil},
		{"size 101 is capped", 101, "", domain.Page{Limit: 100}, nil},
		{"negative size", -1, "", domain.Page{}, domain.ErrInvalidPageSize},
		{"token offset", 10, "20", domain.Page{Limit: 10, Offset: 20}, nil},
		{"token zero", 10, "0", domain.Page{Limit: 10}, nil},
		{"token not a number", 10, "abc", domain.Page{}, domain.ErrInvalidPageToken},
		{"negative token", 10, "-5", domain.Page{}, domain.ErrInvalidPageToken},
		{"token with plus sign", 10, "+5", domain.Page{}, domain.ErrInvalidPageToken},
		{"token with leading zero", 10, "05", domain.Page{}, domain.ErrInvalidPageToken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPage(tt.size, tt.token)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got %+v,%v, want %+v,%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestPage_NextToken(t *testing.T) {
	for _, tt := range []struct {
		name     string
		page     domain.Page
		returned int
		total    int64
		want     string
	}{
		{"more pages", domain.Page{Limit: 20}, 20, 45, "20"},
		{"middle page", domain.Page{Limit: 20, Offset: 20}, 20, 45, "40"},
		{"last partial page", domain.Page{Limit: 20, Offset: 40}, 5, 45, ""},
		{"exactly full last page", domain.Page{Limit: 20, Offset: 20}, 20, 40, ""},
		{"empty result", domain.Page{Limit: 20}, 0, 0, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.page.NextToken(tt.returned, tt.total); got != tt.want {
				t.Fatalf("NextToken()=%q, want %q", got, tt.want)
			}
		})
	}
}
