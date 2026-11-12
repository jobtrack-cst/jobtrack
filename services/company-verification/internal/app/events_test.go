package app_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

const eventID = "55555555-5555-4555-8555-555555555555"

func TestDecisionEvent(t *testing.T) {
	approved := stored(domain.StatusApproved)
	approved.ReviewedBy, approved.UpdatedAt = adminID, now
	rejected := stored(domain.StatusRejected)
	rejected.ReviewedBy, rejected.RejectionReason, rejected.UpdatedAt = adminID, "Expired licence", now

	t.Run("approval is employer.verified", func(t *testing.T) {
		e, err := app.DecisionEvent(approved, eventID)
		require.NoError(t, err)
		require.Equal(t, "employer.verified", e.GetType())
		require.Equal(t, "company-verification", e.GetProducer())
		require.Equal(t, eventID, e.GetEventId())
		require.True(t, e.GetOccurredAt().AsTime().Equal(now), "occurred_at is the decision time")
		var data eventsv1.EmployerVerified
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		require.Equal(t, employerID, data.GetEmployerId())
		// The shared envelope rules accept it, so the outbox can store it.
		_, err = events.MarshalEnvelope(e)
		require.NoError(t, err)
	})
	t.Run("rejection is employer.verification_rejected with the reason", func(t *testing.T) {
		e, err := app.DecisionEvent(rejected, eventID)
		require.NoError(t, err)
		require.Equal(t, "employer.verification_rejected", e.GetType())
		var data eventsv1.EmployerVerificationRejected
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		require.Equal(t, employerID, data.GetEmployerId())
		require.Equal(t, "Expired licence", data.GetReason())
		_, err = events.MarshalEnvelope(e)
		require.NoError(t, err)
	})
	for _, status := range []domain.RequestStatus{domain.StatusPending, domain.StatusUnspecified} {
		t.Run("no event for an undecided request", func(t *testing.T) {
			_, err := app.DecisionEvent(stored(status), eventID)
			require.Error(t, err)
		})
	}
}

// captureDecide stores the event Decide receives so the test can inspect it.
func captureDecide(got **eventsv1.EventEnvelope) func(context.Context, domain.VerificationRequest, *eventsv1.EventEnvelope) (domain.VerificationRequest, error) {
	return func(_ context.Context, r domain.VerificationRequest, e *eventsv1.EventEnvelope) (domain.VerificationRequest, error) {
		*got = e
		return r, nil
	}
}

func TestRequests_Approve_StoresEmployerVerifiedWithDecision(t *testing.T) {
	s, d := newRequests(t)
	var event *eventsv1.EventEnvelope
	d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
	d.clock.EXPECT().Now().Return(now)
	d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureDecide(&event))

	_, err := s.Approve(context.Background(), admin, requestID)
	require.NoError(t, err)
	require.NotNil(t, event)
	require.Equal(t, "employer.verified", event.GetType())
	require.True(t, event.GetOccurredAt().AsTime().Equal(now))
	_, err = uuid.Parse(event.GetEventId())
	require.NoError(t, err, "every event gets a new UUID")
}

func TestRequests_Reject_StoresRejectionWithDecision(t *testing.T) {
	s, d := newRequests(t)
	var event *eventsv1.EventEnvelope
	d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
	d.clock.EXPECT().Now().Return(now)
	d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureDecide(&event))

	_, err := s.Reject(context.Background(), admin, requestID, " Expired licence ")
	require.NoError(t, err)
	require.Equal(t, "employer.verification_rejected", event.GetType())
	var data eventsv1.EmployerVerificationRejected
	require.NoError(t, event.GetData().UnmarshalTo(&data))
	require.Equal(t, "Expired licence", data.GetReason(), "the event carries the trimmed reason")
}

func TestRequests_Decide_EachDecisionGetsItsOwnEventID(t *testing.T) {
	s, d := newRequests(t)
	var first, second *eventsv1.EventEnvelope
	d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil).Times(2)
	d.clock.EXPECT().Now().Return(now).Times(2)
	gomock.InOrder(
		d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureDecide(&first)),
		d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureDecide(&second)),
	)
	_, err := s.Approve(context.Background(), admin, requestID)
	require.NoError(t, err)
	_, err = s.Approve(context.Background(), admin, requestID)
	require.NoError(t, err)
	require.NotEqual(t, first.GetEventId(), second.GetEventId())
}
