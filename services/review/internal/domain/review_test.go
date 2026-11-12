package domain_test

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	appID      = "aaaaaaaa-1111-4222-8333-444455556666"
	employerID = "eeeeeeee-1111-4222-8333-444455556666"
)

func TestNewReviewInput(t *testing.T) {
	tests := []struct {
		name    string
		in      domain.ReviewInput
		wantID  string
		wantErr error
	}{
		// Score boundaries from the contract: 1 to 5.
		{"score 0", domain.ReviewInput{ApplicationID: appID, Score: 0, Decision: domain.DecisionShortlisted}, "", domain.ErrInvalidScore},
		{"score 1", domain.ReviewInput{ApplicationID: appID, Score: 1, Decision: domain.DecisionShortlisted}, appID, nil},
		{"score 3", domain.ReviewInput{ApplicationID: appID, Score: 3, Decision: domain.DecisionRejected}, appID, nil},
		{"score 5", domain.ReviewInput{ApplicationID: appID, Score: 5, Decision: domain.DecisionShortlisted}, appID, nil},
		{"score 6", domain.ReviewInput{ApplicationID: appID, Score: 6, Decision: domain.DecisionShortlisted}, "", domain.ErrInvalidScore},
		{"score negative", domain.ReviewInput{ApplicationID: appID, Score: -1, Decision: domain.DecisionShortlisted}, "", domain.ErrInvalidScore},

		// Decision must be SHORTLISTED or REJECTED; scores do not decide it.
		{"low score shortlisted", domain.ReviewInput{ApplicationID: appID, Score: 1, Decision: domain.DecisionShortlisted}, appID, nil},
		{"high score rejected", domain.ReviewInput{ApplicationID: appID, Score: 5, Decision: domain.DecisionRejected}, appID, nil},
		{"decision unspecified", domain.ReviewInput{ApplicationID: appID, Score: 3, Decision: domain.DecisionUnspecified}, "", domain.ErrInvalidDecision},
		{"decision unknown value", domain.ReviewInput{ApplicationID: appID, Score: 3, Decision: domain.Decision(9)}, "", domain.ErrInvalidDecision},

		// application_id.
		{"uppercase id is lowercased", domain.ReviewInput{ApplicationID: "AAAAAAAA-1111-4222-8333-444455556666", Score: 3, Decision: domain.DecisionShortlisted}, appID, nil},
		{"empty id", domain.ReviewInput{ApplicationID: "", Score: 3, Decision: domain.DecisionShortlisted}, "", domain.ErrInvalidApplicationID},
		{"id not a uuid", domain.ReviewInput{ApplicationID: "application-1", Score: 3, Decision: domain.DecisionShortlisted}, "", domain.ErrInvalidApplicationID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewReviewInput(tt.in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.ValidReviewInput{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.ValidReviewInput{ApplicationID: tt.wantID, Score: tt.in.Score, Decision: tt.in.Decision}, got)
		})
	}
}

func TestApplicationParticipants_CheckOwningEmployer(t *testing.T) {
	p := domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
	tests := []struct {
		name    string
		caller  domain.Caller
		wantErr error
	}{
		{"owning employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, nil},
		{"another employer", domain.Caller{UserID: otherID, Role: domain.RoleEmployer}, domain.ErrNotOwningEmployer},
		{"the applicant", domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}, domain.ErrNotOwningEmployer},
		{"employer ID with applicant role", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, domain.ErrNotOwningEmployer},
		{"admin", domain.Caller{UserID: employerID, Role: domain.RoleAdmin}, domain.ErrNotOwningEmployer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.CheckOwningEmployer(tt.caller)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestDecision_StringAndParse(t *testing.T) {
	for d, s := range map[domain.Decision]string{domain.DecisionShortlisted: "SHORTLISTED", domain.DecisionRejected: "REJECTED"} {
		assert.Equal(t, s, d.String())
		got, ok := domain.ParseDecision(s)
		require.True(t, ok)
		assert.Equal(t, d, got)
	}
	for _, bad := range []string{"", "shortlisted", "UNSPECIFIED"} {
		got, ok := domain.ParseDecision(bad)
		assert.False(t, ok)
		assert.Equal(t, domain.DecisionUnspecified, got)
	}
	assert.Equal(t, "", domain.DecisionUnspecified.String())
}
