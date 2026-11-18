package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestJobs_RecordJobPlansAllMatchesAndUniqueNotifications(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo, tx := mocks.NewMockJobRepository(ctrl), mocks.NewMockJobTransaction(ctrl)
	repo.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, apply func(ports.JobTransaction) error) error { return apply(tx) })
	job := &eventsv1.JobPublished{JobId: category, Location: "Thimphu"}
	candidates := []domain.Alert{
		{ID: "c", ApplicantID: category, Location: "Thimphu"},
		{ID: "b", ApplicantID: owner, Location: "Thimphu"},
		{ID: "a", ApplicantID: owner, Location: "Thimphu"},
		{ID: "d", ApplicantID: owner, Location: "Paro"},
	}
	now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now)
	gomock.InOrder(
		tx.EXPECT().Claim(gomock.Any(), "source").Return(true, nil),
		tx.EXPECT().Candidates(gomock.Any(), domain.PublishedJob{ID: category, Location: "Thimphu"}).Return(candidates, nil),
		tx.EXPECT().SaveMatches(gomock.Any(), "source", category, []domain.Match{{AlertID: "a", ApplicantID: owner}, {AlertID: "b", ApplicantID: owner}, {AlertID: "c", ApplicantID: category}}).Return(nil),
	)
	ids, applicants := []string{}, []string{}
	tx.EXPECT().AddEvent(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, e *eventsv1.EventEnvelope) error {
		require.Equal(t, "job_alert.matched", e.Type)
		require.Equal(t, "job-alert", e.Producer)
		require.Equal(t, now, e.OccurredAt.AsTime())
		_, err := events.MarshalEnvelope(e)
		require.NoError(t, err)
		var data eventsv1.JobAlertMatched
		require.NoError(t, e.Data.UnmarshalTo(&data))
		require.Equal(t, category, data.JobId)
		ids = append(ids, e.EventId)
		applicants = append(applicants, data.ApplicantId)
		return nil
	})
	require.NoError(t, app.NewJobs(repo, clock).RecordJob(context.Background(), "source", job))
	require.Equal(t, []string{owner, category}, applicants)
	require.NotEqual(t, ids[0], ids[1])
}
func TestJobs_RecordJobDuplicatesAndFailures(t *testing.T) {
	failure := errors.New("store failure")
	for _, stage := range []string{"duplicate", "claim", "candidates", "matches", "event", "no match"} {
		t.Run(stage, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo, tx := mocks.NewMockJobRepository(ctrl), mocks.NewMockJobTransaction(ctrl)
			repo.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, apply func(ports.JobTransaction) error) error { return apply(tx) })
			claimed := stage != "duplicate"
			var claimErr error
			if stage == "claim" {
				claimErr = failure
			}
			tx.EXPECT().Claim(gomock.Any(), "source").Return(claimed, claimErr)
			if claimed && claimErr == nil {
				candidates := []domain.Alert{{ID: category, ApplicantID: owner, Location: "Thimphu"}}
				if stage == "no match" {
					candidates = nil
				}
				var candidateErr error
				if stage == "candidates" {
					candidateErr = failure
				}
				tx.EXPECT().Candidates(gomock.Any(), gomock.Any()).Return(candidates, candidateErr)
				if candidateErr == nil {
					var matchErr error
					if stage == "matches" {
						matchErr = failure
					}
					tx.EXPECT().SaveMatches(gomock.Any(), "source", category, gomock.Any()).Return(matchErr)
					if matchErr == nil && stage != "no match" {
						tx.EXPECT().AddEvent(gomock.Any(), gomock.Any()).Return(failure)
					}
				}
			}
			err := app.NewJobs(repo, nil).RecordJob(context.Background(), "source", &eventsv1.JobPublished{JobId: category, Location: "Thimphu"})
			if stage == "duplicate" || stage == "no match" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}
