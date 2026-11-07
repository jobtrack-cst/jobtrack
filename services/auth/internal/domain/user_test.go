package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
)

func TestRole_StringAndParse(t *testing.T) {
	for _, tt := range []struct {
		name  string
		role  domain.Role
		claim string
	}{
		{"applicant", domain.RoleApplicant, "applicant"},
		{"employer", domain.RoleEmployer, "employer"},
		{"admin", domain.RoleAdmin, "admin"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.String(); got != tt.claim {
				t.Errorf("String()=%q, want %q", got, tt.claim)
			}
			got, ok := domain.ParseRole(tt.claim)
			if !ok || got != tt.role {
				t.Errorf("ParseRole(%q)=%v,%v, want %v,true", tt.claim, got, ok, tt.role)
			}
		})
	}
}

func TestParseRole_RejectsUnknownValues(t *testing.T) {
	for _, s := range []string{"", "Admin", "ADMIN", "superuser", " applicant"} {
		if _, ok := domain.ParseRole(s); ok {
			t.Errorf("ParseRole(%q) accepted an unknown role", s)
		}
	}
}

func TestRole_CanSelfRegister(t *testing.T) {
	for _, tt := range []struct {
		role domain.Role
		want bool
	}{
		{domain.RoleUnspecified, false},
		{domain.RoleApplicant, true},
		{domain.RoleEmployer, true},
		{domain.RoleAdmin, false},
		{domain.Role(99), false},
	} {
		if got := tt.role.CanSelfRegister(); got != tt.want {
			t.Errorf("Role(%d).CanSelfRegister()=%v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	// local@ + domain, sized to hit the 254 boundary exactly.
	at254 := strings.Repeat("a", 64) + "@" + strings.Repeat("b", 186) + ".bt"
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"valid", "pema@cst.edu.bt", "pema@cst.edu.bt", false},
		{"trims and lowercases", "  Pema@CST.edu.BT ", "pema@cst.edu.bt", false},
		{"empty", "", "", true},
		{"spaces only", "   ", "", true},
		{"no at sign", "pema.cst.edu.bt", "", true},
		{"display name form", "Pema <pema@cst.edu.bt>", "", true},
		{"254 characters", at254, at254, false},
		{"255 characters", "a" + at254, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeEmail(tt.in)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidEmail) {
					t.Fatalf("err=%v, want ErrInvalidEmail", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q,%v, want %q,nil", got, err, tt.want)
			}
		})
	}
}

func TestNormalizeFullName(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"valid", "Pema Wangmo", "Pema Wangmo", false},
		{"trims", "  Pema  ", "Pema", false},
		{"empty", "", "", true},
		{"spaces only", "   ", "", true},
		{"1 character", "P", "P", false},
		{"100 characters", strings.Repeat("a", 100), strings.Repeat("a", 100), false},
		{"101 characters", strings.Repeat("a", 101), "", true},
		{"100 multibyte characters", strings.Repeat("ཀ", 100), strings.Repeat("ཀ", 100), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeFullName(tt.in)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidFullName) {
					t.Fatalf("err=%v, want ErrInvalidFullName", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q,%v, want %q,nil", got, err, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	for _, tt := range []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"7 characters", "abcdefg", true},
		{"8 characters", "abcdefgh", false},
		{"72 bytes", strings.Repeat("a", 72), false},
		{"73 bytes", strings.Repeat("a", 73), true},
		{"spaces kept", "  pass word  ", false},
		{"empty", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidatePassword(tt.password)
			if tt.wantErr != errors.Is(err, domain.ErrInvalidPassword) || (!tt.wantErr && err != nil) {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminPassword(t *testing.T) {
	for _, tt := range []struct {
		name     string
		password string
		want     error
	}{
		{"missing", "", domain.ErrWeakAdminPassword},
		{"placeholder", domain.AdminPasswordPlaceholder, domain.ErrWeakAdminPassword},
		{"11 characters", strings.Repeat("a", 11), domain.ErrWeakAdminPassword},
		{"12 characters", strings.Repeat("a", 12), nil},
		{"73 bytes", strings.Repeat("a", 73), domain.ErrInvalidPassword},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateAdminPassword(tt.password); !errors.Is(err, tt.want) {
				t.Fatalf("err=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewRegistration(t *testing.T) {
	const email, password, name = "pema@cst.edu.bt", "password123", "Pema"
	for _, tt := range []struct {
		name     string
		email    string
		password string
		fullName string
		role     domain.Role
		want     error
	}{
		{"applicant", email, password, name, domain.RoleApplicant, nil},
		{"employer", email, password, name, domain.RoleEmployer, nil},
		{"admin cannot self-register", email, password, name, domain.RoleAdmin, domain.ErrRoleNotAllowed},
		{"unspecified role", email, password, name, domain.RoleUnspecified, domain.ErrRoleNotAllowed},
		{"bad email", "nope", password, name, domain.RoleApplicant, domain.ErrInvalidEmail},
		{"bad name", email, password, " ", domain.RoleApplicant, domain.ErrInvalidFullName},
		{"short password", email, "short", name, domain.RoleApplicant, domain.ErrInvalidPassword},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := domain.NewRegistration(tt.email, tt.password, tt.fullName, tt.role)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err=%v, want %v", err, tt.want)
			}
			if tt.want == nil && (reg.Email != email || reg.FullName != name || reg.Role != tt.role) {
				t.Fatalf("unexpected registration %+v", reg)
			}
		})
	}
}
