package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

func TestNewProfileFields(t *testing.T) {
	// "ä" is 2 bytes, so these check that limits count characters, not bytes.
	name150 := strings.Repeat("ä", 150)
	desc2000 := strings.Repeat("d", 2000)
	loc100 := strings.Repeat("l", 100)
	site255 := "https://" + strings.Repeat("w", 243) + ".bt" + "/" // 8 + 243 + 3 + 1 = 255
	tests := []struct {
		name                         string
		companyName, desc, site, loc string
		want                         domain.ProfileFields
		wantErr                      error
	}{
		{name: "all fields", companyName: "Druk Tech", desc: "Software", site: "https://druk.bt", loc: "Thimphu",
			want: domain.ProfileFields{CompanyName: "Druk Tech", Description: "Software", Website: "https://druk.bt", Location: "Thimphu"}},
		{name: "only company name", companyName: "Druk Tech", want: domain.ProfileFields{CompanyName: "Druk Tech"}},
		{name: "trims every field", companyName: "  Druk Tech ", desc: " Software ", site: " http://druk.bt ", loc: " Paro ",
			want: domain.ProfileFields{CompanyName: "Druk Tech", Description: "Software", Website: "http://druk.bt", Location: "Paro"}},
		// company_name boundaries: 0, 1, 150, 151
		{name: "empty company name", companyName: "", wantErr: domain.ErrInvalidCompanyName},
		{name: "spaces-only company name", companyName: "   ", wantErr: domain.ErrInvalidCompanyName},
		{name: "1-character company name", companyName: "D", want: domain.ProfileFields{CompanyName: "D"}},
		{name: "150-character company name", companyName: name150, want: domain.ProfileFields{CompanyName: name150}},
		{name: "151-character company name", companyName: name150 + "ä", wantErr: domain.ErrInvalidCompanyName},
		// description boundaries: 2000, 2001
		{name: "2000-character description", companyName: "D", desc: desc2000, want: domain.ProfileFields{CompanyName: "D", Description: desc2000}},
		{name: "2001-character description", companyName: "D", desc: desc2000 + "d", wantErr: domain.ErrInvalidDescription},
		// location boundaries: 100, 101
		{name: "100-character location", companyName: "D", loc: loc100, want: domain.ProfileFields{CompanyName: "D", Location: loc100}},
		{name: "101-character location", companyName: "D", loc: loc100 + "l", wantErr: domain.ErrInvalidLocation},
		// website boundaries and format
		{name: "255-character website", companyName: "D", site: site255, want: domain.ProfileFields{CompanyName: "D", Website: site255}},
		{name: "256-character website", companyName: "D", site: site255 + "x", wantErr: domain.ErrInvalidWebsite},
		{name: "website without scheme", companyName: "D", site: "druk.bt", wantErr: domain.ErrInvalidWebsite},
		{name: "website with ftp scheme", companyName: "D", site: "ftp://druk.bt", wantErr: domain.ErrInvalidWebsite},
		{name: "website javascript scheme", companyName: "D", site: "javascript:alert(1)", wantErr: domain.ErrInvalidWebsite},
		{name: "website without host", companyName: "D", site: "https://", wantErr: domain.ErrInvalidWebsite},
		{name: "website with space in host", companyName: "D", site: "https://druk tech.bt", wantErr: domain.ErrInvalidWebsite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewProfileFields(tt.companyName, tt.desc, tt.site, tt.loc)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %+v,%v, want %+v,nil", got, err, tt.want)
			}
		})
	}
}

func TestNewCompanyProfile_StartsUnverifiedInUTC(t *testing.T) {
	now := time.Date(2026, 11, 7, 18, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	f := domain.ProfileFields{CompanyName: "Druk Tech", Location: "Thimphu"}
	p := domain.NewCompanyProfile(employerID, f, now)
	if p.VerificationStatus != domain.VerificationUnverified {
		t.Errorf("status=%v, want VerificationUnverified", p.VerificationStatus)
	}
	if p.RejectionReason != "" {
		t.Errorf("rejection reason=%q, want empty", p.RejectionReason)
	}
	if p.EmployerID != employerID || p.CompanyName != "Druk Tech" || p.Location != "Thimphu" {
		t.Errorf("fields not copied: %+v", p)
	}
	if p.CreatedAt.Location() != time.UTC || !p.CreatedAt.Equal(now) || !p.UpdatedAt.Equal(p.CreatedAt) {
		t.Errorf("timestamps=%v,%v, want both %v in UTC", p.CreatedAt, p.UpdatedAt, now.UTC())
	}
}

func TestCompanyProfile_WithFields_KeepsVerificationAndCreatedAt(t *testing.T) {
	created := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	later := created.Add(time.Hour)
	p := domain.CompanyProfile{
		EmployerID: employerID, CompanyName: "Old", Description: "old", Website: "https://old.bt", Location: "Paro",
		VerificationStatus: domain.VerificationRejected, RejectionReason: "missing licence",
		CreatedAt: created, UpdatedAt: created,
	}
	got := p.WithFields(domain.ProfileFields{CompanyName: "New"}, later)
	want := domain.CompanyProfile{
		EmployerID: employerID, CompanyName: "New",
		VerificationStatus: domain.VerificationRejected, RejectionReason: "missing licence",
		CreatedAt: created, UpdatedAt: later,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCompanyProfile_ViewFor_HidesRejectionReason(t *testing.T) {
	p := domain.CompanyProfile{EmployerID: employerID, VerificationStatus: domain.VerificationRejected, RejectionReason: "missing licence"}
	for _, tt := range []struct {
		name   string
		caller domain.Caller
		want   string
	}{
		{"owning employer sees reason", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, "missing licence"},
		{"admin sees reason", domain.Caller{UserID: otherID, Role: domain.RoleAdmin}, "missing licence"},
		{"other employer does not", domain.Caller{UserID: otherID, Role: domain.RoleEmployer}, ""},
		{"applicant does not", domain.Caller{UserID: otherID, Role: domain.RoleApplicant}, ""},
		{"applicant with owner's ID does not", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := p.ViewFor(tt.caller)
			if got.RejectionReason != tt.want {
				t.Errorf("reason=%q, want %q", got.RejectionReason, tt.want)
			}
			if got.VerificationStatus != domain.VerificationRejected {
				t.Errorf("status changed to %v", got.VerificationStatus)
			}
		})
	}
	if p.RejectionReason != "missing licence" {
		t.Error("ViewFor changed the original profile")
	}
}
