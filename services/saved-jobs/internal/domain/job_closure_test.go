package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

func TestNewJobClosure(t *testing.T) {
	const eventID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2"
	const jobID = "33333333-3333-4333-8333-333333333333"
	closedAt := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		eventID  string
		jobID    string
		closedAt time.Time
		wantErr  bool
	}{
		{"valid", eventID, jobID, closedAt, false},
		{"uppercase IDs are stored lowercase", strings.ToUpper(eventID), strings.ToUpper(jobID), closedAt, false},
		{"time in another zone is stored in UTC", eventID, jobID, closedAt.In(time.FixedZone("BTT", 6*3600)), false},
		{"empty event ID", "", jobID, closedAt, true},
		{"event ID not a UUID", "event-1", jobID, closedAt, true},
		{"empty job ID", eventID, "", closedAt, true},
		{"job ID not a UUID", eventID, "job-1", closedAt, true},
		{"job ID with braces", eventID, "{" + jobID + "}", closedAt, true},
		{"zero closed time", eventID, jobID, time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewJobClosure(tt.eventID, tt.jobID, tt.closedAt)
			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrInvalidJobClosedEvent)
				return
			}
			require.NoError(t, err)
			require.Equal(t, eventID, got.EventID)
			require.Equal(t, jobID, got.JobID)
			require.True(t, closedAt.Equal(got.ClosedAt))
			require.Equal(t, time.UTC, got.ClosedAt.Location())
		})
	}
}
