package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits, in characters after trimming (AGENTS.md Section 16, mirrors
// job-posting: title 1..150, location up to 100). The database columns
// enforce the same lengths.
const (
	MaxTitleLen    = 150
	MaxLocationLen = 100
	// MaxQueryLen bounds the free-text search term. Same as title to keep
	// the LIKE pattern small; noted in the PR.
	MaxQueryLen = 150
	DefaultPageSize = 20
	MaxPageSize     = 100
)

var (
	ErrInvalidQuery      = errors.New("query must be at most 150 characters")
	ErrInvalidCategoryID = errors.New("category_id must be empty or a UUID")
	ErrInvalidTagID      = errors.New("each tag_id must be a UUID")
	ErrInvalidLocation   = errors.New("location must be at most 100 characters")
	ErrInvalidPageSize   = errors.New("page_size must be between 0 and 100")
	ErrInvalidPageToken  = errors.New("page_token must be a non-negative offset")
)

// JobSummary is one open job in the read model. It carries only the fields
// published by the job events (proto JobSummary).
type JobSummary struct {
	JobID      string
	EmployerID string
	Title      string
	CategoryID string
	TagIDs     []string
	Location   string
	Deadline   time.Time
}

// IsOpen reports whether a job is visible in search: it must be published
// (the read model only stores published jobs, so presence implies published)
// and its deadline must be strictly after now.
func (j JobSummary) IsOpen(now time.Time) bool {
	return j.Deadline.After(now)
}

// Filters are the validated, normalized SearchJobs inputs.
type Filters struct {
	Query      string   // lowercased, trimmed; empty means no title filter
	CategoryID string   // empty means no filter
	TagIDs     []string // empty means no tag filter; otherwise OR
	Location   string   // trimmed; empty means no filter
	PageSize   int
	PageOffset int
}

// NewFilters validates raw request values and returns normalized filters.
// uuid is injected so the domain does not import a UUID library directly.
// now is unused today but kept for symmetry with tests of time-based rules.
func NewFilters(query, categoryID string, tagIDs []string, location string, pageSize int, pageToken string, isUUID func(string) bool) (Filters, error) {
	f := Filters{PageSize: pageSize, PageOffset: 0}

	q := strings.TrimSpace(query)
	if utf8.RuneCountInString(q) > MaxQueryLen {
		return Filters{}, ErrInvalidQuery
	}
	f.Query = strings.ToLower(q)

	cat := strings.TrimSpace(categoryID)
	if cat != "" && !isUUID(cat) {
		return Filters{}, ErrInvalidCategoryID
	}
	f.CategoryID = cat

	seen := make(map[string]struct{}, len(tagIDs))
	normalized := make([]string, 0, len(tagIDs))
	for _, t := range tagIDs {
		tt := strings.TrimSpace(t)
		if tt == "" || !isUUID(tt) {
			return Filters{}, ErrInvalidTagID
		}
		if _, ok := seen[tt]; ok {
			continue
		}
		seen[tt] = struct{}{}
		normalized = append(normalized, tt)
	}
	f.TagIDs = normalized

	loc := strings.TrimSpace(location)
	if utf8.RuneCountInString(loc) > MaxLocationLen {
		return Filters{}, ErrInvalidLocation
	}
	f.Location = loc

	if pageSize == 0 {
		f.PageSize = DefaultPageSize
	} else if pageSize < 0 || pageSize > MaxPageSize {
		return Filters{}, ErrInvalidPageSize
	}

	if pageToken != "" {
		offset, err := parseOffset(pageToken)
		if err != nil {
			return Filters{}, ErrInvalidPageToken
		}
		f.PageOffset = offset
	}
	return f, nil
}

// parseOffset parses a decimal non-negative offset; the contract's shared
// pagination uses an opaque token, and this service encodes it as the offset.
func parseOffset(token string) (int, error) {
	var n int
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, ErrInvalidPageToken
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 { // guard against overflow / abusive offsets
			return 0, ErrInvalidPageToken
		}
	}
	return n, nil
}