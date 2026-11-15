package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

const outcomeID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3"

var occurred = time.Date(2026, 11, 12, 18, 5, 0, 0, time.FixedZone("BTT", 6*3600))

func validInput(change func(*app.RecordEventInput)) app.RecordEventInput {
	in := app.RecordEventInput{EventID: outcomeID, EventType: domain.EventStatusChanged, NewStatus: "UNDER_REVIEW", OccurredAt: occurred}
	change(&in)
	return in
}

func TestAnalytics_RecordEvent_StoresValidEvent(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   app.RecordEventInput
		want domain.OutcomeEvent
	}{
		{"job published has no status",
			validInput(func(in *app.RecordEventInput) { in.EventType, in.NewStatus = domain.EventJobPublished, "" }),
			domain.OutcomeEvent{EventID: outcomeID, EventType: domain.EventJobPublished, OccurredAt: occurred.UTC()}},
		{"status change is trimmed and stored in UTC",
			validInput(func(in *app.RecordEventInput) { in.NewStatus = " UNDER_REVIEW " }),
			domain.OutcomeEvent{EventID: outcomeID, EventType: domain.EventStatusChanged, NewStatus: "UNDER_REVIEW", OccurredAt: occurred.UTC()}},
		{"status of 32 characters",
			validInput(func(in *app.RecordEventInput) { in.NewStatus = strings.Repeat("S", 32) }),
			domain.OutcomeEvent{EventID: outcomeID, EventType: domain.EventStatusChanged, NewStatus: strings.Repeat("S", 32), OccurredAt: occurred.UTC()}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)
			ctx := context.Background()
			repo.EXPECT().RecordEvent(ctx, tt.want).Return(true, nil).Times(1)
			recorded, err := svc.RecordEvent(ctx, tt.in)
			require.NoError(t, err)
			require.True(t, recorded)
		})
	}
}

func TestAnalytics_RecordEvent_DuplicateIsNotAnError(t *testing.T) {
	svc, repo := newService(t)
	repo.EXPECT().RecordEvent(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	recorded, err := svc.RecordEvent(context.Background(), validInput(func(*app.RecordEventInput) {}))
	require.NoError(t, err)
	require.False(t, recorded)
}

func TestAnalytics_RecordEvent_RejectsInvalidEvents(t *testing.T) {
	for _, tt := range []struct {
		name    string
		change  func(*app.RecordEventInput)
		wantErr error
	}{
		{"event ID not a UUID", func(in *app.RecordEventInput) { in.EventID = "event-1" }, domain.ErrInvalidEventID},
		{"type not counted", func(in *app.RecordEventInput) { in.EventType = "job.updated" }, domain.ErrInvalidEventType},
		{"status on another event type", func(in *app.RecordEventInput) { in.EventType = domain.EventOfferMade }, domain.ErrInvalidStatus},
		{"status change without status", func(in *app.RecordEventInput) { in.NewStatus = "" }, domain.ErrInvalidStatus},
		{"status of only spaces", func(in *app.RecordEventInput) { in.NewStatus = "   " }, domain.ErrInvalidStatus},
		{"status of 33 characters", func(in *app.RecordEventInput) { in.NewStatus = strings.Repeat("S", 33) }, domain.ErrInvalidStatus},
		{"missing occurred_at", func(in *app.RecordEventInput) { in.OccurredAt = time.Time{} }, domain.ErrInvalidOccurredAt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newService(t) // the mock fails the test if the repository is called
			recorded, err := svc.RecordEvent(context.Background(), validInput(tt.change))
			require.ErrorIs(t, err, domain.ErrInvalidOutcomeEvent)
			require.ErrorIs(t, err, tt.wantErr)
			require.False(t, recorded)
		})
	}
}

func TestAnalytics_RecordEvent_RepositoryErrorIsRetryable(t *testing.T) {
	svc, repo := newService(t)
	dbErr := errors.New("connection refused")
	repo.EXPECT().RecordEvent(gomock.Any(), gomock.Any()).Return(false, dbErr)
	_, err := svc.RecordEvent(context.Background(), validInput(func(*app.RecordEventInput) {}))
	require.ErrorIs(t, err, dbErr)
	require.NotErrorIs(t, err, domain.ErrInvalidOutcomeEvent)
}
