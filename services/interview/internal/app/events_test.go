package app_test

import (
	"os"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
)

const (
	fixtureInterviewID   = "88888888-8888-4888-8888-888888888801"
	fixtureApplicationID = "11111111-1111-4111-8111-111111111111"
)

func TestInterviewScheduledEvent_MatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/contract/fixtures/interview.scheduled.json")
	require.NoError(t, err)
	want, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	var data eventsv1.InterviewScheduled
	require.NoError(t, want.GetData().UnmarshalTo(&data))
	got, err := app.InterviewScheduledEvent(domain.Interview{
		InterviewID: fixtureInterviewID, ApplicationID: fixtureApplicationID,
		ScheduledAt: data.GetScheduledAt().AsTime(), CreatedAt: want.GetOccurredAt().AsTime(),
	}, want.GetEventId())
	require.NoError(t, err)
	require.True(t, proto.Equal(want, got), "built event differs from the fixture")
	encoded, err := events.MarshalEnvelope(got)
	require.NoError(t, err)
	decoded, err := events.UnmarshalEnvelope(encoded)
	require.NoError(t, err)
	require.True(t, proto.Equal(got, decoded))
}

func TestInterviewScheduledEvent_Fields(t *testing.T) {
	thimphu := time.FixedZone("BTT", 6*60*60)
	created := time.Date(2026, 11, 15, 19, 30, 0, 123000, thimphu)
	scheduled := time.Date(2026, 11, 20, 10, 0, 0, 456000, thimphu)
	e, err := app.InterviewScheduledEvent(domain.Interview{
		InterviewID: fixtureInterviewID, ApplicationID: fixtureApplicationID, Details: "Room 3",
		ScheduledAt: scheduled, CreatedAt: created,
	}, "event-1")
	require.NoError(t, err)
	assert.Equal(t, "event-1", e.GetEventId())
	assert.Equal(t, "interview.scheduled", e.GetType())
	assert.Equal(t, "interview", e.GetProducer())
	assert.Equal(t, created.UTC(), e.GetOccurredAt().AsTime())
	var data eventsv1.InterviewScheduled
	require.NoError(t, e.GetData().UnmarshalTo(&data))
	assert.Equal(t, fixtureInterviewID, data.GetInterviewId())
	assert.Equal(t, fixtureApplicationID, data.GetApplicationId())
	assert.Equal(t, scheduled.UTC(), data.GetScheduledAt().AsTime())

	for name, iv := range map[string]domain.Interview{
		"missing scheduled_at": {InterviewID: fixtureInterviewID, ApplicationID: fixtureApplicationID, CreatedAt: created},
		"missing created_at":   {InterviewID: fixtureInterviewID, ApplicationID: fixtureApplicationID, ScheduledAt: scheduled},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := app.InterviewScheduledEvent(iv, "event-1")
			require.Error(t, err)
		})
	}
}
