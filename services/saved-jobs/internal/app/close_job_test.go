package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/ports/mocks"
)

func closureForTest(t *testing.T) domain.JobClosure {
	t.Helper()
	closure, err := domain.NewJobClosure("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2",
		"33333333-3333-4333-8333-333333333333", time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return closure
}

func TestSavedJobs_CloseJob_ReturnsRepositoryOutcome(t *testing.T) {
	tests := []struct {
		name    string
		outcome domain.ClosureOutcome
	}{
		{"bookmarks removed", domain.ClosureOutcome{RemovedBookmarks: 2}},
		{"duplicate event", domain.ClosureOutcome{Duplicate: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(gomock.NewController(t))
			closure := closureForTest(t)
			repo.EXPECT().CloseJob(gomock.Any(), closure).Return(tt.outcome, nil)
			got, err := app.NewSavedJobs(repo).CloseJob(context.Background(), closure)
			require.NoError(t, err)
			require.Equal(t, tt.outcome, got)
		})
	}
}

func TestSavedJobs_CloseJob_ReturnsRepositoryError(t *testing.T) {
	repo := mocks.NewMockRepository(gomock.NewController(t))
	errDB := errors.New("database down")
	repo.EXPECT().CloseJob(gomock.Any(), gomock.Any()).Return(domain.ClosureOutcome{}, errDB)
	_, err := app.NewSavedJobs(repo).CloseJob(context.Background(), closureForTest(t))
	require.ErrorIs(t, err, errDB)
}
