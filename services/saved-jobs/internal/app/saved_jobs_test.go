package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

const validUUID = "11111111-1111-4111-8111-111111111111"

func TestSavedJobs_SaveJob(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		jobID     string
		setupMock func(*mocks.MockRepository)
		wantErr   error
	}{
		{
			name:  "success",
			role:  "applicant",
			jobID: validUUID,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().SaveJob(gomock.Any(), validUUID, validUUID).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:    "wrong role",
			role:    "employer",
			jobID:   validUUID,
			wantErr: domain.ErrApplicantOnly,
		},
		{
			name:    "invalid job id",
			role:    "applicant",
			jobID:   "not-a-uuid",
			wantErr: domain.ErrInvalidJobID,
		},
		{
			name:  "already saved",
			role:  "applicant",
			jobID: validUUID,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().SaveJob(gomock.Any(), validUUID, validUUID).Return(domain.ErrAlreadySaved)
			},
			wantErr: domain.ErrAlreadySaved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockRepository(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}
			svc := app.NewSavedJobs(repo)
			caller, _ := domain.NewCaller(validUUID, tt.role)

			_, err := svc.SaveJob(context.Background(), caller, tt.jobID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SaveJob() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
