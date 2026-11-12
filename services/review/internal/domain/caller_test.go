package domain_test

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
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

func TestCaller_RequireEmployer(t *testing.T) {
	tests := []struct {
		role    domain.Role
		wantErr error
	}{
		{domain.RoleEmployer, nil},
		{domain.RoleApplicant, domain.ErrEmployerRoleRequired},
		{domain.RoleAdmin, domain.ErrEmployerRoleRequired},
		{domain.RoleUnspecified, domain.ErrEmployerRoleRequired},
	}
	for _, tt := range tests {
		t.Run("role "+tt.role.String(), func(t *testing.T) {
			err := domain.Caller{UserID: applicantID, Role: tt.role}.RequireEmployer()
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNormalizeApplicationID(t *testing.T) {
	got, err := domain.NormalizeApplicationID("AAAAAAAA-1111-4222-8333-444455556666")
	require.NoError(t, err)
	assert.Equal(t, "aaaaaaaa-1111-4222-8333-444455556666", got)
	_, err = domain.NormalizeApplicationID("nope")
	require.ErrorIs(t, err, domain.ErrInvalidApplicationID)
}
