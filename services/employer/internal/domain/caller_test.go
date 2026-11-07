package domain_test

import (
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	otherID    = "22222222-2222-4222-8222-222222222222"
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
	for _, s := range []string{"", "Employer", "EMPLOYER", " employer", "superuser"} {
		if _, ok := domain.ParseRole(s); ok {
			t.Errorf("ParseRole(%q) accepted an unknown role", s)
		}
	}
}

func TestNormalizeEmployerID(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"lowercase UUID", employerID, employerID, false},
		{"uppercase UUID is lowercased", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", false},
		{"empty", "", "", true},
		{"not a UUID", "employer-1", "", true},
		{"braces", "{" + employerID + "}", "", true},
		{"no dashes", "11111111111141118111111111111111", "", true},
		{"one character short", employerID[:35], "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeEmployerID(tt.in)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidEmployerID) {
					t.Fatalf("err=%v, want ErrInvalidEmployerID", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q,%v, want %q,nil", got, err, tt.want)
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
		wantErr bool
	}{
		{"employer", employerID, "employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, false},
		{"applicant", employerID, "applicant", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, false},
		{"admin", employerID, "admin", domain.Caller{UserID: employerID, Role: domain.RoleAdmin}, false},
		{"user ID not a UUID", "abc", "employer", domain.Caller{}, true},
		{"empty user ID", "", "employer", domain.Caller{}, true},
		{"unknown role", employerID, "superuser", domain.Caller{}, true},
		{"empty role", employerID, "", domain.Caller{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewCaller(tt.userID, tt.role)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrUnauthenticated) {
					t.Fatalf("err=%v, want ErrUnauthenticated", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %+v,%v, want %+v,nil", got, err, tt.want)
			}
		})
	}
}

func TestCaller_RequireEmployer(t *testing.T) {
	for _, tt := range []struct {
		role    domain.Role
		wantErr error
	}{
		{domain.RoleEmployer, nil},
		{domain.RoleApplicant, domain.ErrEmployerRoleRequired},
		{domain.RoleAdmin, domain.ErrEmployerRoleRequired},
		{domain.RoleUnspecified, domain.ErrEmployerRoleRequired},
	} {
		t.Run(tt.role.String(), func(t *testing.T) {
			err := domain.Caller{UserID: employerID, Role: tt.role}.RequireEmployer()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
		})
	}
}
