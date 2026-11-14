package app_test

import (
	"os"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

const fixtureApplicationID = "11111111-1111-4111-8111-111111111111"

func TestReviewCompletedEvent_MatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/contract/fixtures/review.completed.json")
	require.NoError(t, err)
	want, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	got, err := app.ReviewCompletedEvent(domain.Review{
		ApplicationID: fixtureApplicationID, Decision: domain.DecisionShortlisted, Score: 1,
		CreatedAt: want.GetOccurredAt().AsTime(),
	}, want.GetEventId())
	require.NoError(t, err)
	require.True(t, proto.Equal(want, got), "built event differs from the fixture")
	encoded, err := events.MarshalEnvelope(got)
	require.NoError(t, err)
	decoded, err := events.UnmarshalEnvelope(encoded)
	require.NoError(t, err)
	require.True(t, proto.Equal(got, decoded))
}

func TestReviewCompletedEvent_Fields(t *testing.T) {
	thimphu := time.FixedZone("BTT", 6*60*60)
	created := time.Date(2026, 11, 15, 6, 30, 0, 123000, thimphu)
	for _, tt := range []struct {
		decision domain.Decision
		want     string
	}{{domain.DecisionShortlisted, "SHORTLISTED"}, {domain.DecisionRejected, "REJECTED"}} {
		t.Run(tt.want, func(t *testing.T) {
			// Score 5 rejected and score 1 shortlisted: the score never decides.
			e, err := app.ReviewCompletedEvent(domain.Review{ApplicationID: fixtureApplicationID, Decision: tt.decision, Score: 3, CreatedAt: created}, "event-1")
			require.NoError(t, err)
			assert.Equal(t, "event-1", e.GetEventId())
			assert.Equal(t, "review.completed", e.GetType())
			assert.Equal(t, "review", e.GetProducer())
			assert.Equal(t, created.UTC(), e.GetOccurredAt().AsTime())
			var data eventsv1.ReviewCompleted
			require.NoError(t, e.GetData().UnmarshalTo(&data))
			assert.Equal(t, fixtureApplicationID, data.GetApplicationId())
			assert.Equal(t, tt.want, data.GetDecision())
		})
	}
	t.Run("unspecified decision is refused", func(t *testing.T) {
		_, err := app.ReviewCompletedEvent(domain.Review{ApplicationID: fixtureApplicationID, CreatedAt: created}, "event-1")
		require.Error(t, err)
	})
}
