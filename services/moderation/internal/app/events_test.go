package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

const eventID = "55555555-5555-4555-8555-555555555555"

func TestJobRemovedEvent(t *testing.T) {
	rm := domain.NewRemoval(jobID, adminID, "fake company", now)

	t.Run("carries the job ID and reason from moderation", func(t *testing.T) {
		e, err := app.JobRemovedEvent(rm, eventID)
		require.NoError(t, err)
		require.Equal(t, "moderation.job_removed", e.GetType())
		require.Equal(t, "moderation", e.GetProducer(), "job-posting accepts the event only from moderation")
		require.Equal(t, eventID, e.GetEventId())
		require.True(t, e.GetOccurredAt().AsTime().Equal(now), "occurred_at is the removal time")
		var data eventsv1.ModerationJobRemoved
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		require.Equal(t, jobID, data.GetJobId())
		require.Equal(t, "fake company", data.GetReason())
		// The shared envelope rules accept it, so the outbox can store it.
		_, err = events.MarshalEnvelope(e)
		require.NoError(t, err)
	})
	for _, tt := range []struct {
		name   string
		change func(*domain.Removal)
	}{
		{"no job ID", func(r *domain.Removal) { r.JobID = "" }},
		{"no reason", func(r *domain.Removal) { r.Reason = "" }},
		{"no removal time", func(r *domain.Removal) { r.RemovedAt = time.Time{} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bad := rm
			tt.change(&bad)
			_, err := app.JobRemovedEvent(bad, eventID)
			require.Error(t, err)
		})
	}
}

// captureRemoveJob stores the event RemoveJob receives so the test can inspect it.
func captureRemoveJob(got **eventsv1.EventEnvelope) func(context.Context, domain.Removal, *eventsv1.EventEnvelope) error {
	return func(_ context.Context, _ domain.Removal, e *eventsv1.EventEnvelope) error {
		*got = e
		return nil
	}
}

func TestReports_RemoveJob_StoresJobRemovedWithTheRemoval(t *testing.T) {
	svc, repo, clock := newReports(t)
	var event *eventsv1.EventEnvelope
	clock.EXPECT().Now().Return(now)
	repo.EXPECT().RemoveJob(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureRemoveJob(&event))

	_, err := svc.RemoveJob(context.Background(), admin, jobID, "  fake company ")
	require.NoError(t, err)
	require.NotNil(t, event)
	require.Equal(t, "moderation.job_removed", event.GetType())
	require.True(t, event.GetOccurredAt().AsTime().Equal(now))
	_, err = uuid.Parse(event.GetEventId())
	require.NoError(t, err, "every event gets a new UUID")
	var data eventsv1.ModerationJobRemoved
	require.NoError(t, event.GetData().UnmarshalTo(&data))
	require.Equal(t, jobID, data.GetJobId())
	require.Equal(t, "fake company", data.GetReason(), "the event carries the trimmed reason")
}

func TestReports_RemoveJob_EachRemovalGetsItsOwnEventID(t *testing.T) {
	svc, repo, clock := newReports(t)
	var first, second *eventsv1.EventEnvelope
	clock.EXPECT().Now().Return(now).Times(2)
	gomock.InOrder(
		repo.EXPECT().RemoveJob(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureRemoveJob(&first)),
		repo.EXPECT().RemoveJob(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(captureRemoveJob(&second)),
	)
	_, err := svc.RemoveJob(context.Background(), admin, jobID, "fake company")
	require.NoError(t, err)
	_, err = svc.RemoveJob(context.Background(), admin, jobID, "fake company")
	require.NoError(t, err)
	require.NotEqual(t, first.GetEventId(), second.GetEventId())
}
