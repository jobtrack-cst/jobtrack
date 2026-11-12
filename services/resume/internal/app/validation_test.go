package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidationResults_Apply(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  domain.ValidationStatus
		reason  string
		outcome domain.ValidationOutcome
		err     error
	}{
		{"validated", domain.ValidationStatusValidated, "", domain.ValidationApplied, nil},
		{"rejected", domain.ValidationStatusRejected, "original reason", domain.ValidationApplied, nil},
		{"duplicate", domain.ValidationStatusValidated, "", domain.ValidationDuplicate, nil},
		{"final conflict", domain.ValidationStatusRejected, "reason", domain.ValidationAlreadyFinal, nil},
		{"unknown target", domain.ValidationStatusValidated, "", domain.ValidationOutcomeUnspecified, domain.ErrResumeNotFound},
		{"mismatched applicant", domain.ValidationStatusValidated, "", domain.ValidationOutcomeUnspecified, domain.ErrValidationApplicantMismatch},
		{"database failed", domain.ValidationStatusValidated, "", domain.ValidationOutcomeUnspecified, errors.New("fixed database error")},
		{"invalid input", domain.ValidationStatusPending, "", domain.ValidationOutcomeUnspecified, domain.ErrInvalidValidationResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockValidationRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			result := domain.ValidationResult{EventID: otherID, ResumeID: resumeID, ApplicantID: ownerID, Status: tc.status, Reason: tc.reason}
			if tc.status != domain.ValidationStatusPending {
				clock.EXPECT().Now().Return(now.In(time.FixedZone("UTC+6", 6*60*60)))
				repo.EXPECT().ApplyValidation(gomock.Any(), result, now).Return(tc.outcome, tc.err)
			}
			outcome, err := app.NewValidationResults(repo, clock).Apply(context.Background(), result)
			require.Equal(t, tc.outcome, outcome)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
