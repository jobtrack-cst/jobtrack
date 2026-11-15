package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// deadline.reminder carries job_id, applicant_id and a deadline within 48
// hours after occurred_at; its event_id is a stable UUIDv5 (JT-82).
func TestDeadlineReminderFixture_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/deadline.reminder.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "deadline.reminder", e.Type)
	require.Equal(t, "deadline-reminder", e.Producer)
	var data eventsv1.DeadlineReminder
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.NotEmpty(t, data.JobId)
	require.NotEmpty(t, data.ApplicantId)
	until := data.Deadline.AsTime().Sub(e.OccurredAt.AsTime())
	require.True(t, until > 0 && until <= 48*60*60*1e9, "deadline within 48 hours of the run")
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
