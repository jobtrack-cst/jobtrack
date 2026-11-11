package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits in characters after trimming. The migration enforces the same
// lengths, so update both together.
const (
	MaxTitleLength       = 150
	MaxDescriptionLength = 5000
	MaxLocationLength    = 100
	MaxTagIDs            = 20
)

// Status is the job lifecycle: DRAFT -> PUBLISHED -> CLOSED. REMOVED is set
// only by the moderation.job_removed consumer (Sprint 2).
type Status uint8

const (
	StatusUnspecified Status = iota
	StatusDraft
	StatusPublished
	StatusClosed
	StatusRemoved
)

// String returns the value stored in the database status column.
func (s Status) String() string {
	switch s {
	case StatusDraft:
		return "draft"
	case StatusPublished:
		return "published"
	case StatusClosed:
		return "closed"
	case StatusRemoved:
		return "removed"
	default:
		return ""
	}
}

// ParseStatus reads a stored status value.
func ParseStatus(s string) (Status, bool) {
	for _, st := range []Status{StatusDraft, StatusPublished, StatusClosed, StatusRemoved} {
		if st.String() == s {
			return st, true
		}
	}
	return StatusUnspecified, false
}

// JobInput is the raw editable content from CreateJob or UpdateJob.
// A zero Deadline means the request had no deadline.
type JobInput struct {
	Title       string
	Description string
	CategoryID  string
	TagIDs      []string
	Location    string
	Deadline    time.Time
}

// JobFields is validated, normalized editable content.
type JobFields struct {
	Title       string
	Description string
	CategoryID  string // empty when not set
	TagIDs      []string
	Location    string
	Deadline    time.Time // UTC, microsecond precision
}

// NewJobFields validates input against the field limits. now is the request
// time; the deadline must be strictly after it.
func NewJobFields(in JobInput, now time.Time) (JobFields, error) {
	title, ok := cleanText(in.Title, MaxTitleLength)
	if !ok || title == "" {
		return JobFields{}, ErrInvalidTitle
	}
	description, ok := cleanText(in.Description, MaxDescriptionLength)
	if !ok {
		return JobFields{}, ErrInvalidDescription
	}
	categoryID := ""
	if in.CategoryID != "" {
		if categoryID, ok = normalizeUUID(in.CategoryID); !ok {
			return JobFields{}, ErrInvalidCategoryID
		}
	}
	tagIDs, err := normalizeTagIDs(in.TagIDs)
	if err != nil {
		return JobFields{}, err
	}
	location, ok := cleanText(in.Location, MaxLocationLength)
	if !ok {
		return JobFields{}, ErrInvalidLocation
	}
	if in.Deadline.IsZero() {
		return JobFields{}, ErrInvalidDeadline
	}
	// PostgreSQL stores microseconds; truncating first means the stored value
	// is exactly what was validated and returned.
	deadline := in.Deadline.UTC().Truncate(time.Microsecond)
	if !deadline.After(now) {
		return JobFields{}, ErrInvalidDeadline
	}
	return JobFields{
		Title: title, Description: description, CategoryID: categoryID,
		TagIDs: tagIDs, Location: location, Deadline: deadline,
	}, nil
}

// cleanText trims s and checks it is valid UTF-8 without NUL and at most max characters.
func cleanText(s string, max int) (string, bool) {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", false
	}
	s = strings.TrimSpace(s)
	return s, utf8.RuneCountInString(s) <= max
}

// normalizeTagIDs checks the count before removing duplicates, so a request
// can never send more than MaxTagIDs entries.
func normalizeTagIDs(raw []string) ([]string, error) {
	if len(raw) > MaxTagIDs {
		return nil, ErrInvalidTagIDs
	}
	ids := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, r := range raw {
		id, ok := normalizeUUID(r)
		if !ok {
			return nil, ErrInvalidTagIDs
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Job is one vacancy. EmployerID is the owning employer's user ID from auth.
type Job struct {
	ID         string
	EmployerID string
	JobFields
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CheckOwner allows only the employer who created the job.
func (j Job) CheckOwner(c Caller) error {
	if c.Role != RoleEmployer || c.UserID != j.EmployerID {
		return ErrNotJobOwner
	}
	return nil
}

// CheckEditable allows UpdateJob only in DRAFT or PUBLISHED.
func (j Job) CheckEditable() error {
	if j.Status != StatusDraft && j.Status != StatusPublished {
		return ErrJobNotEditable
	}
	return nil
}

// CheckPublishable allows PublishJob only for a DRAFT job whose deadline is
// still in the future. Employer verification is checked separately in app.
func (j Job) CheckPublishable(now time.Time) error {
	if j.Status != StatusDraft {
		return ErrJobNotDraft
	}
	if !j.Deadline.After(now) {
		return ErrDeadlinePassed
	}
	return nil
}

// CheckClosable allows CloseJob only for a PUBLISHED job.
func (j Job) CheckClosable() error {
	if j.Status != StatusPublished {
		return ErrJobNotPublished
	}
	return nil
}

// VisibleTo hides DRAFT and REMOVED jobs from everyone except the owner and admins.
func (j Job) VisibleTo(c Caller) bool {
	switch j.Status {
	case StatusPublished, StatusClosed:
		return true
	default:
		return c.Role == RoleAdmin || (c.Role == RoleEmployer && c.UserID == j.EmployerID)
	}
}

// IsOpen is the application open check: PUBLISHED and deadline after now.
func (j Job) IsOpen(now time.Time) bool {
	return j.Status == StatusPublished && j.Deadline.After(now)
}
