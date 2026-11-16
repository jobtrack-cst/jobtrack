package contract_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

// moderation.job_removed carries job_id and the takedown reason (1 to 500
// characters, JT-34). job-posting's consumer (JT-68) accepts it only from the
// producer moderation with a UUID job_id (JT-87).
func TestModerationFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/moderation.job_removed.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "moderation.job_removed", e.Type)
	require.Equal(t, "moderation", e.Producer)
	require.Regexp(t, fixtureUUID, e.GetEventId())
	var data eventsv1.ModerationJobRemoved
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.Regexp(t, fixtureUUID, data.GetJobId())
	require.NotEmpty(t, data.GetReason())
	require.LessOrEqual(t, len([]rune(data.GetReason())), 500)
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
