package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

func TestBuildSummary(t *testing.T) {
	tests := []struct {
		name   string
		counts []domain.EventCount
		want   domain.Summary
	}{
		{"no rows gives all zeros", nil, domain.Summary{}},
		{
			"each type goes to its own counter",
			[]domain.EventCount{
				{EventType: domain.EventJobPublished, Count: 1},
				{EventType: domain.EventJobClosed, Count: 2},
				{EventType: domain.EventApplicationSubmitted, Count: 3},
				{EventType: domain.EventApplicationWithdrawn, Count: 4},
				{EventType: domain.EventOfferMade, Count: 5},
				{EventType: domain.EventOfferAccepted, Count: 6},
				{EventType: domain.EventOfferDeclined, Count: 7},
			},
			domain.Summary{
				JobsPublished: 1, JobsClosed: 2, ApplicationsSubmitted: 3, ApplicationsWithdrawn: 4,
				OffersMade: 5, OffersAccepted: 6, OffersDeclined: 7,
			},
		},
		{
			"status changes are sorted by status name",
			[]domain.EventCount{
				{EventType: domain.EventStatusChanged, NewStatus: "UNDER_REVIEW", Count: 4},
				{EventType: domain.EventStatusChanged, NewStatus: "OFFERED", Count: 1},
				{EventType: domain.EventStatusChanged, NewStatus: "SHORTLISTED", Count: 2},
			},
			domain.Summary{StatusChanges: []domain.StatusCount{
				{Status: "OFFERED", Count: 1}, {Status: "SHORTLISTED", Count: 2}, {Status: "UNDER_REVIEW", Count: 4},
			}},
		},
		{
			"zero and negative counts are left out",
			[]domain.EventCount{
				{EventType: domain.EventJobPublished, Count: 0},
				{EventType: domain.EventStatusChanged, NewStatus: "REJECTED", Count: 0},
				{EventType: domain.EventStatusChanged, NewStatus: "WITHDRAWN", Count: -1},
			},
			domain.Summary{},
		},
		{
			"repeated rows are added together",
			[]domain.EventCount{
				{EventType: domain.EventOfferMade, Count: 2},
				{EventType: domain.EventOfferMade, Count: 3},
				{EventType: domain.EventStatusChanged, NewStatus: "OFFERED", Count: 1},
				{EventType: domain.EventStatusChanged, NewStatus: "OFFERED", Count: 2},
			},
			domain.Summary{OffersMade: 5, StatusChanges: []domain.StatusCount{{Status: "OFFERED", Count: 3}}},
		},
		{
			"types analytics does not count are ignored",
			[]domain.EventCount{{EventType: "user.registered", Count: 9}, {EventType: domain.EventJobClosed, Count: 1}},
			domain.Summary{JobsClosed: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.BuildSummary(tt.counts))
		})
	}
}
