package domain_test

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	employerID = "3f2b8c1e-1111-4222-8333-444455556666"
	otherID    = "9a8b7c6d-1111-4222-8333-444455556666"
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
		{"employer", employerID, "employer", employerID, domain.RoleEmployer, nil},
		{"applicant", employerID, "applicant", employerID, domain.RoleApplicant, nil},
		{"admin", employerID, "admin", employerID, domain.RoleAdmin, nil},
		{"uppercase id is lowercased", "3F2B8C1E-1111-4222-8333-444455556666", "employer", employerID, domain.RoleEmployer, nil},
		{"empty id", "", "employer", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"id not a uuid", "not-a-uuid", "employer", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"empty role", employerID, "", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"unknown role", employerID, "superuser", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
		{"role wrong case", employerID, "Employer", "", domain.RoleUnspecified, domain.ErrUnauthenticated},
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
		t.Run(tt.role.String(), func(t *testing.T) {
			err := domain.Caller{UserID: employerID, Role: tt.role}.RequireEmployer()
			assert.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeJobID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{"valid", employerID, employerID, nil},
		{"uppercase", "3F2B8C1E-1111-4222-8333-444455556666", employerID, nil},
		{"empty", "", "", domain.ErrInvalidJobID},
		{"not a uuid", "job-1", "", domain.ErrInvalidJobID},
		{"braces", "{" + employerID + "}", "", domain.ErrInvalidJobID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeJobID(tt.raw)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
