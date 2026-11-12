package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports/mocks"
)

const verificationEventID = "8a1f0c52-3d4e-4b6a-9c7d-1e2f3a4b5c6d"

var decidedAt = time.Date(2026, 11, 12, 4, 30, 0, 0, time.UTC)

func TestVerifications_Apply(t *testing.T) {
	valid := domain.VerificationDecision{
		EventID: verificationEventID, EmployerID: employerID,
		Status: domain.VerificationRejected, RejectionReason: reason, DecidedAt: decidedAt,
	}
	tests := []struct {
		name     string
		decision domain.VerificationDecision
		repoOut  domain.VerificationOutcome
		repoErr  error
		callRepo bool
		wantErr  error
	}{
		{name: "applied", decision: valid, repoOut: domain.OutcomeApplied, callRepo: true},
		{name: "duplicate is not an error", decision: valid, repoOut: domain.OutcomeDuplicate, callRepo: true},
		{name: "stale is not an error", decision: valid, repoOut: domain.OutcomeStale, callRepo: true},
		{name: "stored before the profile exists", decision: valid, repoOut: domain.OutcomeStoredWithoutProfile, callRepo: true},
		{name: "database error is returned for a retry", decision: valid, repoErr: errDB, callRepo: true, wantErr: errDB},
		{name: "invalid decision never reaches the database",
			decision: domain.VerificationDecision{EventID: verificationEventID, EmployerID: "bad", Status: domain.VerificationVerified, DecidedAt: decidedAt},
			wantErr:  domain.ErrInvalidVerificationEvent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo, clock := mocks.NewMockVerificationRepository(ctrl), mocks.NewMockClock(ctrl)
			if tt.callRepo {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().ApplyVerification(gomock.Any(), valid, now).Return(tt.repoOut, tt.repoErr)
			}
			got, err := app.NewVerifications(repo, clock).Apply(context.Background(), tt.decision)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.repoOut, got)
		})
	}
}
