package domain

import (
	"errors"
	"strconv"
)

// Pagination rule agreed with Phuntsho on 6 Nov 2026 (to be added to AGENTS.md).
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

var (
	ErrInvalidPageSize  = errors.New("page_size must not be negative")
	ErrInvalidPageToken = errors.New("page_token is not valid")
)

// Page is an offset window. The page token is the offset as a decimal string.
type Page struct {
	Limit  int
	Offset int
}

// NewPage uses the default for 0 and caps sizes above the maximum.
func NewPage(pageSize int32, pageToken string) (Page, error) {
	if pageSize < 0 {
		return Page{}, ErrInvalidPageSize
	}
	limit := int(pageSize)
	if limit == 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	offset := 0
	if pageToken != "" {
		n, err := strconv.Atoi(pageToken)
		// The round trip rejects forms like "+5" or "007".
		if err != nil || n < 0 || strconv.Itoa(n) != pageToken {
			return Page{}, ErrInvalidPageToken
		}
		offset = n
	}
	return Page{Limit: limit, Offset: offset}, nil
}

// NextToken is empty on the last page.
func (p Page) NextToken(returned int, total int64) string {
	next := p.Offset + returned
	if returned == 0 || int64(next) >= total {
		return ""
	}
	return strconv.Itoa(next)
}
