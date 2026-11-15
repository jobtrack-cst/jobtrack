package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// interview.scheduled carries interview_id, application_id and scheduled_at,
// with scheduled_at after occurred_at (JT-80).
func TestInterviewFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/interview.scheduled.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "interview.scheduled", e.Type)
	require.Equal(t, "interview", e.Producer)
	var data eventsv1.InterviewScheduled
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.NotEmpty(t, data.InterviewId)
	require.NotEmpty(t, data.ApplicationId)
	require.True(t, data.ScheduledAt.AsTime().After(e.OccurredAt.AsTime()))
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
