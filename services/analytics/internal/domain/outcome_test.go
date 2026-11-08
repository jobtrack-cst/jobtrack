package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

const eventID = "8a1c2b3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"

func TestNewOutcomeEvent(t *testing.T) {
	occurred := time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	const changed = domain.EventStatusChanged
	tests := []struct {
		name       string
		eventID    string
		eventType  string
		status     string
		occurredAt time.Time
		wantStatus string
		wantErr    error
	}{
		{"job.published", eventID, domain.EventJobPublished, "", occurred, "", nil},
		{"job.closed", eventID, domain.EventJobClosed, "", occurred, "", nil},
		{"application.submitted", eventID, domain.EventApplicationSubmitted, "", occurred, "", nil},
		{"application.withdrawn", eventID, domain.EventApplicationWithdrawn, "", occurred, "", nil},
		{"offer.made", eventID, domain.EventOfferMade, "", occurred, "", nil},
		{"offer.accepted", eventID, domain.EventOfferAccepted, "", occurred, "", nil},
		{"offer.declined", eventID, domain.EventOfferDeclined, "", occurred, "", nil},
		{"status change", eventID, changed, "SHORTLISTED", occurred, "SHORTLISTED", nil},

		{"uppercase event ID is lowercased", strings.ToUpper(eventID), domain.EventJobPublished, "", occurred, "", nil},
		{"event ID not a UUID", "123", domain.EventJobPublished, "", occurred, "", domain.ErrInvalidEventID},
		{"empty event ID", "", domain.EventJobPublished, "", occurred, "", domain.ErrInvalidEventID},

		{"type analytics does not count", eventID, "user.registered", "", occurred, "", domain.ErrInvalidEventType},
		{"uppercase type", eventID, "Job.Published", "", occurred, "", domain.ErrInvalidEventType},
		{"padded type", eventID, " job.published", "", occurred, "", domain.ErrInvalidEventType},
		{"empty type", eventID, "", "", occurred, "", domain.ErrInvalidEventType},

		{"status is trimmed", eventID, changed, "  UNDER_REVIEW \t", occurred, "UNDER_REVIEW", nil},
		{"status 1 character", eventID, changed, "X", occurred, "X", nil},
		{"status 32 characters", eventID, changed, strings.Repeat("A", 32), occurred, strings.Repeat("A", 32), nil},
		{"status 33 characters", eventID, changed, strings.Repeat("A", 33), occurred, "", domain.ErrInvalidStatus},
		{"limit counts characters: 32 two-byte characters", eventID, changed, strings.Repeat("é", 32), occurred, strings.Repeat("é", 32), nil},
		{"limit counts characters: 33 two-byte characters", eventID, changed, strings.Repeat("é", 33), occurred, "", domain.ErrInvalidStatus},
		{"32 characters after trimming", eventID, changed, " " + strings.Repeat("A", 32) + " ", occurred, strings.Repeat("A", 32), nil},
		{"empty status on a status change", eventID, changed, "", occurred, "", domain.ErrInvalidStatus},
		{"spaces-only status", eventID, changed, "   ", occurred, "", domain.ErrInvalidStatus},
		{"status with NUL", eventID, changed, "OFF\x00ERED", occurred, "", domain.ErrInvalidStatus},
		{"status with invalid UTF-8", eventID, changed, "OFFERED\xff", occurred, "", domain.ErrInvalidStatus},
		{"status on another event type", eventID, domain.EventOfferMade, "OFFERED", occurred, "", domain.ErrInvalidStatus},
		{"spaces-only status on another event type", eventID, domain.EventJobClosed, " ", occurred, "", domain.ErrInvalidStatus},

		{"zero occurred_at", eventID, domain.EventJobPublished, "", time.Time{}, "", domain.ErrInvalidOccurredAt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewOutcomeEvent(tt.eventID, tt.eventType, tt.status, tt.occurredAt)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.OutcomeEvent{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, strings.ToLower(tt.eventID), got.EventID)
			assert.Equal(t, tt.eventType, got.EventType)
			assert.Equal(t, tt.wantStatus, got.NewStatus)
			assert.Equal(t, time.UTC, got.OccurredAt.Location())
			assert.True(t, tt.occurredAt.Equal(got.OccurredAt))
		})
	}
}

func TestIsCountedEventType(t *testing.T) {
	counted := []string{
		"job.published", "job.closed", "application.submitted", "application.withdrawn",
		"application.status_changed", "offer.made", "offer.accepted", "offer.declined",
	}
	for _, s := range counted {
		assert.True(t, domain.IsCountedEventType(s), s)
	}
	for _, s := range []string{"", "job.updated", "user.registered", "message.sent", "JOB.PUBLISHED", "job.published "} {
		assert.False(t, domain.IsCountedEventType(s), s)
	}
}
