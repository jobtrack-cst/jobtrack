package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
)

func TestPeriod_IsTheSevenDaysBeforeNowInUTC(t *testing.T) {
	thimphu := time.FixedZone("BTT", 6*60*60)
	now := time.Date(2026, 11, 16, 8, 0, 0, 123456789, thimphu)
	start, end := domain.Period(now)
	assert.Equal(t, time.Date(2026, 11, 16, 2, 0, 0, 123456000, time.UTC), end)
	assert.Equal(t, time.UTC, end.Location())
	assert.Equal(t, domain.Window, end.Sub(start))
	assert.Equal(t, time.Date(2026, 11, 9, 2, 0, 0, 123456000, time.UTC), start)
}

func TestSubject_ShowsBothDates(t *testing.T) {
	for _, tt := range []struct {
		name       string
		start, end time.Time
		want       string
	}{
		{"same month", time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC), time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC),
			"JobTrack weekly summary (8 Nov 2026 to 15 Nov 2026)"},
		{"across a year", time.Date(2026, 12, 28, 2, 0, 0, 0, time.UTC), time.Date(2027, 1, 4, 2, 0, 0, 0, time.UTC),
			"JobTrack weekly summary (28 Dec 2026 to 4 Jan 2027)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.Subject(domain.Summary{Start: tt.start, End: tt.end}))
		})
	}
}

func TestBody_ListsEveryCountAndStatus(t *testing.T) {
	s := domain.Summary{
		Start: time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC),
		JobsPublished: 3, JobsClosed: 1, ApplicationsSubmitted: 10, ApplicationsWithdrawn: 2,
		OffersMade: 4, OffersAccepted: 2, OffersDeclined: 1,
		StatusChanges: []domain.StatusCount{{Status: "SHORTLISTED", Count: 5}, {Status: "UNDER_REVIEW", Count: 7}},
	}
	want := "JobTrack weekly summary\n" +
		"Period: 8 Nov 2026 02:00 UTC to 15 Nov 2026 02:00 UTC\n\n" +
		"Jobs published: 3\nJobs closed: 1\nApplications submitted: 10\nApplications withdrawn: 2\n" +
		"Offers made: 4\nOffers accepted: 2\nOffers declined: 1\n\n" +
		"Application status changes:\n  SHORTLISTED: 5\n  UNDER_REVIEW: 7\n\n" +
		"This is an automated summary from JobTrack.\n"
	assert.Equal(t, want, domain.Body(s))
}

func TestBody_EdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name     string
		changes  []domain.StatusCount
		contains string
	}{
		{"no status changes", nil, "Application status changes:\n  none\n"},
		{"control characters replaced", []domain.StatusCount{{Status: "BAD\x1b\u0085", Count: 1}}, "  BAD??: 1\n"},
		{"invalid UTF-8 replaced", []domain.StatusCount{{Status: "BAD\xff", Count: 1}}, "  BAD?: 1\n"},
		{"line break cannot add lines", []domain.StatusCount{{Status: "A\nB", Count: 1}}, "  A?B: 1\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, domain.Body(domain.Summary{StatusChanges: tt.changes}), tt.contains)
		})
	}
}
