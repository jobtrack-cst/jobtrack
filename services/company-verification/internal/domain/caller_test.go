package domain_test

import (
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	adminID    = "22222222-2222-4222-8222-222222222222"
	requestID  = "33333333-3333-4333-8333-333333333333"
)

func TestRole_StringAndParse(t *testing.T) {
	for _, tt := range []struct {
		role  domain.Role
		value string
	}{
		{domain.RoleApplicant, "applicant"},
		{domain.RoleEmployer, "employer"},
		{domain.RoleAdmin, "admin"},
	} {
		t.Run(tt.value, func(t *testing.T) {
			if got := tt.role.String(); got != tt.value {
				t.Errorf("String()=%q, want %q", got, tt.value)
			}
			got, ok := domain.ParseRole(tt.value)
			if !ok || got != tt.role {
				t.Errorf("ParseRole(%q)=%v,%v, want %v,true", tt.value, got, ok, tt.role)
			}
		})
	}
	if got := domain.RoleUnspecified.String(); got != "" {
		t.Errorf("RoleUnspecified.String()=%q, want empty", got)
	}
}

func TestParseRole_RejectsUnknownValues(t *testing.T) {
	for _, s := range []string{"", "Admin", "ADMIN", " admin", "superuser"} {
		if _, ok := domain.ParseRole(s); ok {
			t.Errorf("ParseRole(%q) accepted an unknown role", s)
		}
	}
}

func TestNormalizeID(t *testing.T) {
	invalid := errors.New("invalid")
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"lowercase UUID", requestID, requestID, nil},
		{"uppercase UUID is lowercased", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", nil},
		{"empty", "", "", invalid},
		{"not a UUID", "request-1", "", invalid},
		{"braces", "{" + requestID + "}", "", invalid},
		{"no dashes", "33333333333343338333333333333333", "", invalid},
		{"one character short", requestID[:35], "", invalid},
		{"one character long", requestID + "3", "", invalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeID(tt.in, invalid)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got %q,%v, want %q,%v", got, err, tt.want, tt.wantErr)
			}
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
		{"employer", employerID, "employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, nil},
		{"admin", adminID, "admin", domain.Caller{UserID: adminID, Role: domain.RoleAdmin}, nil},
		{"applicant", employerID, "applicant", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, nil},
		{"uppercase user ID is lowercased", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "admin",
			domain.Caller{UserID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Role: domain.RoleAdmin}, nil},
		{"user ID not a UUID", "user-1", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"empty user ID", "", "admin", domain.Caller{}, domain.ErrUnauthenticated},
		{"unknown role", employerID, "superuser", domain.Caller{}, domain.ErrUnauthenticated},
		{"empty role", employerID, "", domain.Caller{}, domain.ErrUnauthenticated},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewCaller(tt.userID, tt.role)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got %+v,%v, want %+v,%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestCaller_RoleChecks(t *testing.T) {
	for _, tt := range []struct {
		role         domain.Role
		wantEmployer error
		wantAdmin    error
	}{
		{domain.RoleEmployer, nil, domain.ErrAdminRoleRequired},
		{domain.RoleAdmin, domain.ErrEmployerRoleRequired, nil},
		{domain.RoleApplicant, domain.ErrEmployerRoleRequired, domain.ErrAdminRoleRequired},
		{domain.RoleUnspecified, domain.ErrEmployerRoleRequired, domain.ErrAdminRoleRequired},
	} {
		t.Run(tt.role.String(), func(t *testing.T) {
			c := domain.Caller{UserID: employerID, Role: tt.role}
			if err := c.RequireEmployer(); !errors.Is(err, tt.wantEmployer) {
				t.Errorf("RequireEmployer()=%v, want %v", err, tt.wantEmployer)
			}
			if err := c.RequireAdmin(); !errors.Is(err, tt.wantAdmin) {
				t.Errorf("RequireAdmin()=%v, want %v", err, tt.wantAdmin)
			}
		})
	}
}
