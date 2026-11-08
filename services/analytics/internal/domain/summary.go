package domain

import (
	"slices"
	"strings"
)

// EventCount is one row of the repository count: how many stored events of
// one type, and for status changes one new_status, fall in the range.
type EventCount struct {
	EventType string
	NewStatus string
	Count     int64
}

// StatusCount is the number of application.status_changed events with one
// new_status.
type StatusCount struct {
	Status string
	Count  int64
}

// Summary holds the counts GetSummary and GetInternalSummary return.
type Summary struct {
	JobsPublished         int64
	JobsClosed            int64
	ApplicationsSubmitted int64
	ApplicationsWithdrawn int64
	OffersMade            int64
	OffersAccepted        int64
	OffersDeclined        int64
	// Only statuses with a count above 0, sorted by status name; nil when
	// there are none.
	StatusChanges []StatusCount
}

// BuildSummary turns repository rows into a Summary. Types with no rows stay
// 0. Rows for other types cannot be stored (the table has a CHECK on the
// eight types), so they are ignored.
func BuildSummary(counts []EventCount) Summary {
	var s Summary
	statuses := map[string]int64{}
	for _, c := range counts {
		if c.Count <= 0 {
			continue
		}
		switch c.EventType {
		case EventJobPublished:
			s.JobsPublished += c.Count
		case EventJobClosed:
			s.JobsClosed += c.Count
		case EventApplicationSubmitted:
			s.ApplicationsSubmitted += c.Count
		case EventApplicationWithdrawn:
			s.ApplicationsWithdrawn += c.Count
		case EventOfferMade:
			s.OffersMade += c.Count
		case EventOfferAccepted:
			s.OffersAccepted += c.Count
		case EventOfferDeclined:
			s.OffersDeclined += c.Count
		case EventStatusChanged:
			statuses[c.NewStatus] += c.Count
		}
	}
	for status, n := range statuses {
		s.StatusChanges = append(s.StatusChanges, StatusCount{Status: status, Count: n})
	}
	slices.SortFunc(s.StatusChanges, func(a, b StatusCount) int { return strings.Compare(a.Status, b.Status) })
	return s
}
