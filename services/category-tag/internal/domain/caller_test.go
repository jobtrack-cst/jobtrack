package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

const userID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

func TestParseRole(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want domain.Role
		ok   bool
	}{
		{"applicant", domain.RoleApplicant, true},
		{"employer", domain.RoleEmployer, true},
		{"admin", domain.RoleAdmin, true},
		{"Admin", domain.RoleUnspecified, false},
		{" admin", domain.RoleUnspecified, false},
		{"", domain.RoleUnspecified, false},
		{"superuser", domain.RoleUnspecified, false},
	} {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := domain.ParseRole(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

func TestNewCaller(t *testing.T) {
	for _, tt := range []struct {
		name    string
		userID  string
		role    string
		want    domain.Caller
		wantErr error
	}{
		{"applicant", userID, "applicant", domain.Caller{UserID: userID, Role: domain.RoleApplicant}, nil},
		{"employer", userID, "employer", domain.Caller{UserID: userID, Role: domain.RoleEmployer}, nil},
		{"admin", userID, "admin", domain.Caller{UserID: userID, Role: domain.RoleAdmin}, nil},
		{"uppercase ID is lowercased", "3F2B8C1E-1A2B-4C3D-8E9F-0A1B2C3D4E5F", "admin", domain.Caller{UserID: userID, Role: domain.RoleAdmin}, nil},
		{"user ID not a UUID", "user-1", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"empty user ID", "", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"unknown role", userID, "guest", domain.Caller{}, domain.ErrUnauthenticated},
		{"empty role", userID, "", domain.Caller{}, domain.ErrUnauthenticated},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewCaller(tt.userID, tt.role)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCaller_RequireAuthenticated(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		wantErr error
	}{
		{"applicant may read", domain.Caller{UserID: userID, Role: domain.RoleApplicant}, nil},
		{"employer may read", domain.Caller{UserID: userID, Role: domain.RoleEmployer}, nil},
		{"admin may read", domain.Caller{UserID: userID, Role: domain.RoleAdmin}, nil},
		{"zero caller", domain.Caller{}, domain.ErrUnauthenticated},
		{"no role", domain.Caller{UserID: userID}, domain.ErrUnauthenticated},
		{"no user ID", domain.Caller{Role: domain.RoleAdmin}, domain.ErrUnauthenticated},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, tt.caller.RequireAuthenticated(), tt.wantErr)
		})
	}
}
