package domain_test

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	applicantID = "3f2b8c1e-1111-4222-8333-444455556666"
	otherID     = "9a8b7c6d-1111-4222-8333-444455556666"
)

func TestNewCaller(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		role     string
		wantID   string
		wantRole domain.Role
		wantErr  error
	}{
		{"applicant", applicantID, "applicant", applicantID, domain.RoleApplicant, nil},
		{"employer", applicantID, "employer", applicantID, domain.RoleEmployer, nil},
		{"admin", applicantID, "admin", applicantID, domain.RoleAdmin, nil},
		{"uppercase id is lowercased", "3F2B8C1E-1111-4222-8333-444455556666", "applicant", applicantID, domain.RoleApplicant, nil},
		{"empty id", "", "applicant", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"id not a uuid", "not-a-uuid", "applicant", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"empty role", applicantID, "", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"unknown role", applicantID, "superuser", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"role wrong case", applicantID, "Applicant", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := domain.NewCaller(tt.userID, tt.role)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, c.UserID)
			assert.Equal(t, tt.wantRole, c.Role)
		})
	}
}

func TestCaller_RequireApplicant(t *testing.T) {
	tests := []struct {
		role    domain.Role
		wantErr error
	}{
		{domain.RoleApplicant, nil},
		{domain.RoleEmployer, domain.ErrApplicantRoleRequired},
		{domain.RoleAdmin, domain.ErrApplicantRoleRequired},
		{domain.RoleUnspecified, domain.ErrApplicantRoleRequired},
	}
	for _, tt := range tests {
		t.Run("role "+tt.role.String(), func(t *testing.T) {
			err := domain.Caller{UserID: applicantID, Role: tt.role}.RequireApplicant()
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNormalizeResumeID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{"valid", applicantID, applicantID, nil},
		{"uppercase", "3F2B8C1E-1111-4222-8333-444455556666", applicantID, nil},
		{"empty", "", "", domain.ErrInvalidResumeID},
		{"not a uuid", "resume-1", "", domain.ErrInvalidResumeID},
		{"missing hyphens", "3f2b8c1e111142228333444455556666", "", domain.ErrInvalidResumeID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeResumeID(tt.raw)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
