package contract_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

func TestApplicationStatusFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/application.status_changed.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "application.status_changed", e.Type)
	require.Equal(t, "application-status", e.Producer)
	var data eventsv1.StatusChanged
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.Equal(t, "SUBMITTED", data.GetPreviousStatus())
	require.Equal(t, "UNDER_REVIEW", data.GetNewStatus())
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
