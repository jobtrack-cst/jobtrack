// Package domain holds the weekly summary and its plain-text email.
package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Window is the period one run reports on: the 7 days before the run.
const Window = 7 * 24 * time.Hour

// Summary holds the counts Analytics returns for one period.
type Summary struct {
	Start, End            time.Time
	JobsPublished         int64
	JobsClosed            int64
	ApplicationsSubmitted int64
	ApplicationsWithdrawn int64
	OffersMade            int64
	OffersAccepted        int64
	OffersDeclined        int64
	StatusChanges         []StatusCount
}

// StatusCount is the number of application.status_changed events for one status.
type StatusCount struct {
	Status string
	Count  int64
}

// Period returns the rolling window that ends at now. Analytics stores
// microseconds, so the end is truncated to keep the reported period exact.
func Period(now time.Time) (start, end time.Time) {
	end = now.UTC().Truncate(time.Microsecond)
	return end.Add(-Window), end
}

// Subject is the email subject, for example
// "JobTrack weekly summary (8 Nov 2026 to 15 Nov 2026)".
func Subject(s Summary) string {
	return fmt.Sprintf("JobTrack weekly summary (%s to %s)", s.Start.UTC().Format("2 Jan 2006"), s.End.UTC().Format("2 Jan 2006"))
}

// Body is the plain-text email body. Times are shown in UTC.
func Body(s Summary) string {
	var b strings.Builder
	const at = "2 Jan 2006 15:04 UTC"
	b.WriteString("JobTrack weekly summary\n")
	fmt.Fprintf(&b, "Period: %s to %s\n\n", s.Start.UTC().Format(at), s.End.UTC().Format(at))
	for _, line := range []struct {
		label string
		count int64
	}{
		{"Jobs published", s.JobsPublished},
		{"Jobs closed", s.JobsClosed},
		{"Applications submitted", s.ApplicationsSubmitted},
		{"Applications withdrawn", s.ApplicationsWithdrawn},
		{"Offers made", s.OffersMade},
		{"Offers accepted", s.OffersAccepted},
		{"Offers declined", s.OffersDeclined},
	} {
		fmt.Fprintf(&b, "%s: %d\n", line.label, line.count)
	}
	b.WriteString("\nApplication status changes:\n")
	if len(s.StatusChanges) == 0 {
		b.WriteString("  none\n")
	}
	for _, c := range s.StatusChanges {
		fmt.Fprintf(&b, "  %s: %d\n", safeText(c.Status), c.Count)
	}
	b.WriteString("\nThis is an automated summary from JobTrack.\n")
	return b.String()
}

// safeText replaces control characters, which email-sender rejects in a body
// (except CR, LF and tab), so one odd status name cannot block every email.
func safeText(s string) string {
	s = strings.ToValidUTF8(s, "?")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}
