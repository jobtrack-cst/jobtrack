package app_test

import (
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

func TestMessageEvent_MatchesApprovedFixture(t *testing.T) {
	body, err := os.ReadFile("../../../../tests/contract/fixtures/message.sent.json")
	require.NoError(t, err)
	fixture, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	var data eventsv1.MessageSent
	require.NoError(t, fixture.Data.UnmarshalTo(&data))
	envelope, err := app.MessageEvent(domain.Message{ID: data.MessageId, ApplicationID: data.ApplicationId, RecipientID: data.RecipientId, Content: "private message content", CreatedAt: fixture.OccurredAt.AsTime()}, fixture.EventId)
	require.NoError(t, err)
	require.True(t, proto.Equal(fixture, envelope))
	encoded, err := events.MarshalEnvelope(envelope)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private message content")
}
