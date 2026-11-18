package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"go.uber.org/mock/gomock"
)

// Helpers and fixtures (newProfiles, stored, employer, now, errDB) are in profiles_test.go.

// appLogo returns n bytes starting with the JPEG signature.
func appLogo(n int) []byte {
	b := make([]byte, n)
	copy(b, []byte{0xff, 0xd8, 0xff})
	return b
}

func TestProfiles_PutLogo(t *testing.T) {
	jpeg := appLogo(100)
	withLogo := func(id string) domain.CompanyProfile {
		p := stored()
		p.EmployerID, p.HasLogo, p.UpdatedAt = id, true, now
		return p
	}
	for _, tt := range []struct {
		name        string
		caller      domain.Caller
		contentType string
		content     []byte
		setup       func(d deps)
		wantReason  string
		wantErr     error
	}{
		{
			name: "owner stores logo keyed by own ID and keeps verification", caller: employer, contentType: "image/jpeg", content: jpeg,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().PutLogo(gomock.Any(), employerID, domain.Logo{ContentType: "image/jpeg", Content: jpeg}, now).
					Return(withLogo(employerID), nil)
			},
			wantReason: reason,
		},
		{
			// The request has no employer_id, so another employer only reaches their own logo.
			name: "another employer only reaches their own profile", caller: other, contentType: "image/jpeg", content: jpeg,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().PutLogo(gomock.Any(), otherID, gomock.Any(), now).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
		{name: "applicant is denied", caller: appl, contentType: "image/jpeg", content: jpeg, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin is denied", caller: admin, contentType: "image/jpeg", content: jpeg, wantErr: domain.ErrEmployerRoleRequired},
		{name: "role is checked before input", caller: appl, contentType: "image/gif", wantErr: domain.ErrEmployerRoleRequired},
		{name: "empty content is rejected", caller: employer, contentType: "image/jpeg", wantErr: domain.ErrInvalidLogoSize},
		{name: "1,000,001 bytes is rejected", caller: employer, contentType: "image/jpeg", content: appLogo(1_000_001), wantErr: domain.ErrInvalidLogoSize},
		{name: "GIF is rejected", caller: employer, contentType: "image/gif", content: jpeg, wantErr: domain.ErrInvalidLogoType},
		{name: "wrong signature is rejected", caller: employer, contentType: "image/png", content: jpeg, wantErr: domain.ErrInvalidLogoContent},
		{
			name: "repository error is passed through", caller: employer, contentType: "image/jpeg", content: jpeg,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().PutLogo(gomock.Any(), employerID, gomock.Any(), now).Return(domain.CompanyProfile{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.PutLogo(context.Background(), tt.caller, tt.contentType, tt.content)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.HasLogo || got.VerificationStatus != domain.VerificationRejected || got.RejectionReason != tt.wantReason {
				t.Fatalf("got %+v, want own profile with logo, unchanged verification and reason %q", got, tt.wantReason)
			}
		})
	}
}

func TestProfiles_GetLogo(t *testing.T) {
	logo := domain.Logo{ContentType: "image/jpeg", Content: appLogo(10)}
	for _, tt := range []struct {
		name       string
		caller     domain.Caller
		employerID string
		setup      func(d deps)
		wantErr    error
	}{
		{
			name: "applicant reads any employer's logo", caller: appl, employerID: employerID,
			setup: func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(logo, nil) },
		},
		{
			name: "admin reads any employer's logo", caller: admin, employerID: employerID,
			setup: func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(logo, nil) },
		},
		{
			name: "another employer reads the logo", caller: other, employerID: employerID,
			setup: func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(logo, nil) },
		},
		{
			name: "uppercase ID is normalized", caller: appl, employerID: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
			setup: func(d deps) {
				d.repo.EXPECT().GetLogo(gomock.Any(), "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa").Return(logo, nil)
			},
		},
		{name: "ID that is not a UUID", caller: appl, employerID: "abc", wantErr: domain.ErrInvalidEmployerID},
		{name: "empty ID", caller: appl, employerID: "", wantErr: domain.ErrInvalidEmployerID},
		{
			name: "no logo or no profile", caller: appl, employerID: otherID,
			setup: func(d deps) {
				d.repo.EXPECT().GetLogo(gomock.Any(), otherID).Return(domain.Logo{}, domain.ErrLogoNotFound)
			},
			wantErr: domain.ErrLogoNotFound,
		},
		{
			name: "repository error is passed through", caller: appl, employerID: employerID,
			setup:   func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(domain.Logo{}, errDB) },
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.GetLogo(context.Background(), tt.caller, tt.employerID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.ContentType != "image/jpeg" || len(got.Content) != 10 {
				t.Fatalf("got %q with %d bytes,%v, want the stored JPEG", got.ContentType, len(got.Content), err)
			}
		})
	}
}
