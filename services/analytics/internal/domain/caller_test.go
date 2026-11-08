package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

const userID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

func TestParseRole(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   domain.Role
		wantOK bool
	}{
		{"applicant", "applicant", domain.RoleApplicant, true},
		{"employer", "employer", domain.RoleEmployer, true},
		{"admin", "admin", domain.RoleAdmin, true},
		{"uppercase is rejected", "Admin", domain.RoleUnspecified, false},
		{"padded is rejected", " admin", domain.RoleUnspecified, false},
		{"empty is rejected", "", domain.RoleUnspecified, false},
		{"unknown is rejected", "analyst", domain.RoleUnspecified, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := domain.ParseRole(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeID(t *testing.T) {
	invalid := errors.New("invalid")
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"lowercase UUID", userID, userID, nil},
		{"uppercase UUID is lowercased", "3F2B8C1E-1A2B-4C3D-8E9F-0A1B2C3D4E5F", userID, nil},
		{"empty", "", "", invalid},
		{"no hyphens", "3f2b8c1e1a2b4c3d8e9f0a1b2c3d4e5f", "", invalid},
		{"one character short", userID[:35], "", invalid},
		{"one character long", userID + "0", "", invalid},
		{"non hex character", "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5g", "", invalid},
		{"surrounding spaces", " " + userID + " ", "", invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeID(tt.input, invalid)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewCaller(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		role    string
		want    domain.Caller
		wantErr error
	}{
		{"applicant", userID, "applicant", domain.Caller{UserID: userID, Role: domain.RoleApplicant}, nil},
		{"employer", userID, "employer", domain.Caller{UserID: userID, Role: domain.RoleEmployer}, nil},
		{"admin", userID, "admin", domain.Caller{UserID: userID, Role: domain.RoleAdmin}, nil},
		{"uppercase user ID is lowercased", "3F2B8C1E-1A2B-4C3D-8E9F-0A1B2C3D4E5F", "admin", domain.Caller{UserID: userID, Role: domain.RoleAdmin}, nil},
		{"missing user ID", "", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"user ID not a UUID", "not-a-uuid", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"missing role", userID, "", domain.Caller{}, domain.ErrUnauthenticated},
		{"unknown role", userID, "analyst", domain.Caller{}, domain.ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewCaller(tt.userID, tt.role)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Caller{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCaller_RequireAdmin(t *testing.T) {
	tests := []struct {
		name    string
		role    domain.Role
		wantErr error
	}{
		{"admin is allowed", domain.RoleAdmin, nil},
		{"employer is denied", domain.RoleEmployer, domain.ErrAdminRoleRequired},
		{"applicant is denied", domain.RoleApplicant, domain.ErrAdminRoleRequired},
		{"unspecified is denied", domain.RoleUnspecified, domain.ErrAdminRoleRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.Caller{UserID: userID, Role: tt.role}.RequireAdmin()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
