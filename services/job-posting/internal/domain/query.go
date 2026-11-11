package domain

import (
	"strconv"
	"time"
)

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

// NextPageToken is empty on the last page. It compares without adding
// Offset and Limit first, so a huge page_token cannot overflow into a
// negative token; total is never negative and Limit is at most 100.
func (p Page) NextPageToken(total int) string {
	if p.Offset >= total-p.Limit {
		return ""
	}
	return strconv.Itoa(p.Offset + p.Limit)
}

// ClosingWindow is the [Start, End) deadline range for ListJobsClosingBetween.
type ClosingWindow struct {
	Start time.Time
	End   time.Time
}

// NewClosingWindow requires both times with start before end. Both bounds are
// rounded up to a whole microsecond, the precision PostgreSQL stores (as in analytics).
func NewClosingWindow(start, end time.Time) (ClosingWindow, error) {
	if start.IsZero() || end.IsZero() {
		return ClosingWindow{}, ErrInvalidTimeRange
	}
	s, e := ceilMicro(start.UTC()), ceilMicro(end.UTC())
	if !s.Before(e) {
		return ClosingWindow{}, ErrInvalidTimeRange
	}
	return ClosingWindow{Start: s, End: e}, nil
}

func ceilMicro(t time.Time) time.Time {
	truncated := t.Truncate(time.Microsecond)
	if truncated.Before(t) {
		return truncated.Add(time.Microsecond)
	}
	return truncated
}
