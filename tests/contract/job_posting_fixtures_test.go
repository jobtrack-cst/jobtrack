package contract_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

func TestJobPostingFixtures_SharedEnvelope(t *testing.T) {
	for _, tt := range []struct {
		key  string
		data proto.Message
	}{{"job.published", &eventsv1.JobPublished{}}, {"job.closed", &eventsv1.JobClosed{}}} {
		t.Run(tt.key, func(t *testing.T) {
			raw, err := os.ReadFile("fixtures/" + tt.key + ".json")
			require.NoError(t, err)
			e, err := events.UnmarshalEnvelope(raw)
			require.NoError(t, err)
			require.Equal(t, tt.key, e.Type)
			require.Equal(t, "job-posting", e.Producer)
			require.NoError(t, e.Data.UnmarshalTo(tt.data))
			b, err := events.MarshalEnvelope(e)
			require.NoError(t, err)
			again, err := events.UnmarshalEnvelope(b)
			require.NoError(t, err)
			require.True(t, proto.Equal(e, again))
		})
	}
}
