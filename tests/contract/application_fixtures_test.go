package contract_test

import (
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

func TestApplicationFixtures_SharedEnvelope(t *testing.T) {
	for _, tt := range []struct {
		key  string
		data proto.Message
	}{{"application.submitted", &eventsv1.ApplicationSubmitted{}}, {"application.withdrawn", &eventsv1.ApplicationWithdrawn{}}} {
		t.Run(tt.key, func(t *testing.T) {
			raw, err := os.ReadFile("fixtures/" + tt.key + ".json")
			require.NoError(t, err)
			e, err := events.UnmarshalEnvelope(raw)
			require.NoError(t, err)
			require.Equal(t, tt.key, e.Type)
			require.Equal(t, "application", e.Producer)
			require.NoError(t, e.Data.UnmarshalTo(tt.data))
			b, err := events.MarshalEnvelope(e)
			require.NoError(t, err)
			again, err := events.UnmarshalEnvelope(b)
			require.NoError(t, err)
			require.True(t, proto.Equal(e, again))
		})
	}
}
