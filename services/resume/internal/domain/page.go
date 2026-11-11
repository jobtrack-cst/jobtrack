package domain

import "strconv"

// Pagination rules from AGENTS.md Section 9.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Page is a validated limit and offset.
type Page struct {
	Limit  int
	Offset int
}

// NewPage applies the shared pagination rules: 0 means 20, above 100 is
// capped, negative is invalid; the token is an offset as a decimal string.
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
		// Digits only, so "+5", "-1" and " 5" are rejected.
		for _, r := range pageToken {
			if r < '0' || r > '9' {
				return Page{}, ErrInvalidPageToken
			}
		}
		n, err := strconv.Atoi(pageToken)
		if err != nil {
			return Page{}, ErrInvalidPageToken
		}
		offset = n
	}
	return Page{Limit: limit, Offset: offset}, nil
}

// NextPageToken is empty on the last page.
func (p Page) NextPageToken(total int) string {
	next := p.Offset + p.Limit
	if next >= total {
		return ""
	}
	return strconv.Itoa(next)
}
