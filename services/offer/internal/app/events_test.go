package app_test

import (
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

func TestOfferEvents_MatchSharedFixtures(t *testing.T) {
	for _, tt := range []struct {
		key    string
		status domain.OfferStatus
	}{{"made", domain.OfferStatusPending}, {"accepted", domain.OfferStatusAccepted}, {"declined", domain.OfferStatusDeclined}} {
		t.Run(tt.key, func(t *testing.T) {
			body, err := os.ReadFile("../../../../tests/contract/fixtures/offer." + tt.key + ".json")
			require.NoError(t, err)
			want, err := events.UnmarshalEnvelope(body)
			require.NoError(t, err)
			got, err := app.OfferEvent(domain.Offer{ID: "88888888-8888-4888-8888-888888888888", ApplicationID: "11111111-1111-4111-8111-111111111111", ApplicantID: "22222222-2222-4222-8222-222222222222", Status: tt.status, UpdatedAt: want.OccurredAt.AsTime()}, want.EventId)
			require.NoError(t, err)
			require.True(t, proto.Equal(want, got))
			encoded, err := events.MarshalEnvelope(got)
			require.NoError(t, err)
			decoded, err := events.UnmarshalEnvelope(encoded)
			require.NoError(t, err)
			require.True(t, proto.Equal(decoded, got))
		})
	}
}
