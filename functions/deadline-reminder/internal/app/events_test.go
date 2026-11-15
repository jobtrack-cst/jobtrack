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

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/app"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

func TestReminderEvent_MatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/contract/fixtures/deadline.reminder.json")
	require.NoError(t, err)
	want, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	var data eventsv1.DeadlineReminder
	require.NoError(t, want.GetData().UnmarshalTo(&data))
	r, err := domain.NewReminder(data.GetJobId(), data.GetApplicantId(), data.GetDeadline().AsTime())
	require.NoError(t, err)
	got, err := app.ReminderEvent(r, want.GetOccurredAt().AsTime())
	require.NoError(t, err)
	require.True(t, proto.Equal(want, got), "built event differs from the fixture (including the stable event_id)")
	encoded, err := events.MarshalEnvelope(got)
	require.NoError(t, err)
	decoded, err := events.UnmarshalEnvelope(encoded)
	require.NoError(t, err)
	require.True(t, proto.Equal(got, decoded))
}

func TestReminderEvent_Fields(t *testing.T) {
	deadline := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	r, err := domain.NewReminder("33333333-3333-4333-8333-333333333333", "22222222-2222-4222-8222-222222222222", deadline)
	require.NoError(t, err)
	runAt := time.Date(2026, 11, 16, 2, 0, 0, 999, time.FixedZone("BTT", 6*3600))
	e, err := app.ReminderEvent(r, runAt)
	require.NoError(t, err)
	assert.Equal(t, r.EventID(), e.GetEventId())
	assert.Equal(t, "deadline.reminder", e.GetType())
	assert.Equal(t, "deadline-reminder", e.GetProducer())
	assert.Equal(t, runAt.UTC().Truncate(time.Microsecond), e.GetOccurredAt().AsTime())
	var data eventsv1.DeadlineReminder
	require.NoError(t, e.GetData().UnmarshalTo(&data))
	assert.Equal(t, r.JobID, data.GetJobId())
	assert.Equal(t, r.ApplicantID, data.GetApplicantId())
	assert.Equal(t, deadline, data.GetDeadline().AsTime())

	later, err := app.ReminderEvent(r, runAt.Add(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, e.GetEventId(), later.GetEventId(), "the next day's run republishes the same event_id")
}
