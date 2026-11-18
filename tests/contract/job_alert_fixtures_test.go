package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// job_alert.matched uses the approved notification payload (JT-76).
func TestJobAlertFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/job_alert.matched.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "job_alert.matched", e.Type)
	require.Equal(t, "job-alert", e.Producer)
	var data eventsv1.JobAlertMatched
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.Equal(t, "33333333-3333-4333-8333-333333333333", data.JobId)
	require.Equal(t, "55555555-5555-4555-8555-555555555555", data.ApplicantId)
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
