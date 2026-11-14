package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// review.completed carries decision SHORTLISTED or REJECTED (JT-79).
func TestReviewFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/review.completed.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "review.completed", e.Type)
	require.Equal(t, "review", e.Producer)
	var data eventsv1.ReviewCompleted
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.Contains(t, []string{"SHORTLISTED", "REJECTED"}, data.Decision)
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
