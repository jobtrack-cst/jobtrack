package contract_test

import (
	"os"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// message.sent uses the approved notification payload (JT-75).
func TestMessagingFixtures_SharedEnvelope(t *testing.T) {
	raw, err := os.ReadFile("fixtures/message.sent.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, "message.sent", e.Type)
	require.Equal(t, "messaging", e.Producer)
	var data eventsv1.MessageSent
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.Equal(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", data.MessageId)
	require.Equal(t, "11111111-1111-4111-8111-111111111111", data.ApplicationId)
	require.Equal(t, "55555555-5555-4555-8555-555555555555", data.RecipientId)
	b, err := events.MarshalEnvelope(e)
	require.NoError(t, err)
	again, err := events.UnmarshalEnvelope(b)
	require.NoError(t, err)
	require.True(t, proto.Equal(e, again))
}
