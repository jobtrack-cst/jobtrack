package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

const (
	eventID   = "8a1c2b3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	eventType = "job.published"
	producer  = "job-posting"
	dataJSON  = `{"job_id":"3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f","title":"Intern"}`
)

// jsonOfBytes builds a JSON object of exactly n bytes: {"k":"xxx"} has 8
// bytes of overhead around the value.
func jsonOfBytes(n int) string {
	return `{"k":"` + strings.Repeat("x", n-8) + `"}`
}

func TestNewEntry(t *testing.T) {
	occurred := time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	tests := []struct {
		name       string
		eventID    string
		eventType  string
		producer   string
		occurredAt time.Time
		dataJSON   string
		wantErr    error
	}{
		{"valid entry", eventID, eventType, producer, occurred, dataJSON, nil},
		{"uppercase event ID", strings.ToUpper(eventID), eventType, producer, occurred, dataJSON, nil},
		{"event ID not a UUID", "123", eventType, producer, occurred, dataJSON, domain.ErrInvalidEventID},
		{"empty event ID", "", eventType, producer, occurred, dataJSON, domain.ErrInvalidEventID},

		{"event type with underscore", eventID, "job_alert.matched", producer, occurred, dataJSON, nil},
		{"event type with three parts", eventID, "a.b.c", producer, occurred, dataJSON, nil},
		{"event type 100 characters", eventID, "a." + strings.Repeat("b", 98), producer, occurred, dataJSON, nil},
		{"event type 101 characters", eventID, "a." + strings.Repeat("b", 99), producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"empty event type", eventID, "", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"event type with one part", eventID, "job", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"uppercase event type", eventID, "Job.Published", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"event type with space", eventID, " job.published", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"event type with empty part", eventID, "job..published", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"event type ending in dot", eventID, "job.", producer, occurred, dataJSON, domain.ErrInvalidEventType},
		{"event type with hyphen", eventID, "job-alert.matched", producer, occurred, dataJSON, domain.ErrInvalidEventType},

		{"producer one word", eventID, eventType, "auth", occurred, dataJSON, nil},
		{"producer 50 characters", eventID, eventType, strings.Repeat("a", 50), occurred, dataJSON, nil},
		{"producer 51 characters", eventID, eventType, strings.Repeat("a", 51), occurred, dataJSON, domain.ErrInvalidProducer},
		{"empty producer", eventID, eventType, "", occurred, dataJSON, domain.ErrInvalidProducer},
		{"uppercase producer", eventID, eventType, "Job-Posting", occurred, dataJSON, domain.ErrInvalidProducer},
		{"producer with underscore", eventID, eventType, "job_posting", occurred, dataJSON, domain.ErrInvalidProducer},
		{"producer ending in hyphen", eventID, eventType, "job-", occurred, dataJSON, domain.ErrInvalidProducer},

		{"zero occurred_at", eventID, eventType, producer, time.Time{}, dataJSON, domain.ErrInvalidOccurredAt},

		{"empty JSON object", eventID, eventType, producer, occurred, `{}`, nil},
		{"object with surrounding spaces", eventID, eventType, producer, occurred, " {} ", nil},
		{"escaped NUL is allowed", eventID, eventType, producer, occurred, `{"k":"\u0000"}`, nil},
		{"data 65536 bytes", eventID, eventType, producer, occurred, jsonOfBytes(65536), nil},
		{"data 65537 bytes", eventID, eventType, producer, occurred, jsonOfBytes(65537), domain.ErrInvalidDataJSON},
		{"limit counts bytes: 65536 bytes of 2-byte characters", eventID, eventType, producer, occurred, `{"k":"` + strings.Repeat("é", 32764) + `"}`, nil},
		{"limit counts bytes: 65538 bytes of 2-byte characters", eventID, eventType, producer, occurred, `{"k":"` + strings.Repeat("é", 32765) + `"}`, domain.ErrInvalidDataJSON},
		{"empty data", eventID, eventType, producer, occurred, "", domain.ErrInvalidDataJSON},
		{"spaces only", eventID, eventType, producer, occurred, "   ", domain.ErrInvalidDataJSON},
		{"JSON array", eventID, eventType, producer, occurred, `[]`, domain.ErrInvalidDataJSON},
		{"JSON null", eventID, eventType, producer, occurred, `null`, domain.ErrInvalidDataJSON},
		{"JSON string", eventID, eventType, producer, occurred, `"text"`, domain.ErrInvalidDataJSON},
		{"broken JSON", eventID, eventType, producer, occurred, `{"k":}`, domain.ErrInvalidDataJSON},
		{"two objects", eventID, eventType, producer, occurred, `{}{}`, domain.ErrInvalidDataJSON},
		{"invalid UTF-8", eventID, eventType, producer, occurred, "{\"k\":\"\xff\"}", domain.ErrInvalidDataJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewEntry(tt.eventID, tt.eventType, tt.producer, tt.occurredAt, tt.dataJSON)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Entry{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, strings.ToLower(tt.eventID), got.EventID)
			assert.Equal(t, tt.eventType, got.EventType)
			assert.Equal(t, tt.producer, got.Producer)
			assert.Equal(t, time.UTC, got.OccurredAt.Location())
			assert.True(t, tt.occurredAt.Equal(got.OccurredAt))
			assert.Equal(t, tt.dataJSON, got.DataJSON, "data_json is stored as-is")
		})
	}
}

func TestNewListFilter(t *testing.T) {
	t1 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	t2 := t1.Add(time.Nanosecond)
	before := t1.Add(-time.Hour)
	tests := []struct {
		name      string
		eventType string
		start     *time.Time
		end       *time.Time
		wantErr   error
	}{
		{"no filters", "", nil, nil, nil},
		{"event type only", eventType, nil, nil, nil},
		{"start only", "", &t1, nil, nil},
		{"end only", "", nil, &t1, nil},
		{"start just before end", "", &t1, &t2, nil},
		{"start equal to end", "", &t1, &t1, domain.ErrInvalidTimeRange},
		{"start after end", "", &t1, &before, domain.ErrInvalidTimeRange},
		{"all filters", eventType, &before, &t1, nil},
		{"bad event type", "Job", nil, nil, domain.ErrInvalidEventType},
		{"event type 101 characters", "a." + strings.Repeat("b", 99), nil, nil, domain.ErrInvalidEventType},
		{"padded event type", "job.published ", nil, nil, domain.ErrInvalidEventType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewListFilter(tt.eventType, tt.start, tt.end)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.ListFilter{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.eventType, got.EventType)
			assertSameTime(t, tt.start, got.Start)
			assertSameTime(t, tt.end, got.End)
		})
	}
}

func assertSameTime(t *testing.T, want, got *time.Time) {
	t.Helper()
	if want == nil {
		assert.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	assert.True(t, want.Equal(*got))
	assert.Equal(t, time.UTC, got.Location())
}
