package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

const (
	validUUID  = "11111111-1111-4111-8111-111111111111"
	validUUID2 = "22222222-2222-4222-8222-222222222222"
)

func TestMarkNotificationRead(t *testing.T) {
	tests := []struct {
		name           string
		notificationID string
		setupMock      func(*mocks.MockRepository)
		wantErr        error
	}{
		{
			name:           "success",
			notificationID: validUUID,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().MarkNotificationRead(gomock.Any(), validUUID, validUUID).Return(domain.Notification{}, nil)
			},
			wantErr: nil,
		},
		{
			name:           "invalid id",
			notificationID: "not-a-uuid",
			wantErr:        domain.ErrInvalidNotificationID,
		},
		{
			name:           "not owner",
			notificationID: validUUID,
			setupMock: func(m *mocks.MockRepository) {
				m.EXPECT().MarkNotificationRead(gomock.Any(), validUUID, validUUID).Return(domain.Notification{}, domain.ErrNotOwner)
			},
			wantErr: domain.ErrNotOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockRepository(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}
			svc := app.NewNotifications(repo)
			caller, _ := domain.NewCaller(validUUID, "applicant")

			_, err := svc.MarkNotificationRead(context.Background(), caller, tt.notificationID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
