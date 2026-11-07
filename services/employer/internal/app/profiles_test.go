package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	otherID    = "22222222-2222-4222-8222-222222222222"
	reason     = "missing licence"
)

var (
	errDB    = errors.New("database down")
	created  = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	now      = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	employer = domain.Caller{UserID: employerID, Role: domain.RoleEmployer}
	other    = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	appl     = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	admin    = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	input    = app.ProfileInput{CompanyName: " Druk Tech ", Description: "Software", Website: "https://druk.bt", Location: "Thimphu"}
)

type deps struct {
	repo  *mocks.MockProfileRepository
	clock *mocks.MockClock
}

func newProfiles(t *testing.T) (*app.Profiles, deps) {
	ctrl := gomock.NewController(t)
	d := deps{repo: mocks.NewMockProfileRepository(ctrl), clock: mocks.NewMockClock(ctrl)}
	return app.NewProfiles(d.repo, d.clock), d
}

// stored is a rejected profile, so tests can check rejection_reason visibility.
func stored() domain.CompanyProfile {
	return domain.CompanyProfile{
		EmployerID: employerID, CompanyName: "Druk Tech", Description: "Software", Website: "https://druk.bt", Location: "Thimphu",
		VerificationStatus: domain.VerificationRejected, RejectionReason: reason, CreatedAt: created, UpdatedAt: created,
	}
}

func TestProfiles_Create(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		in      app.ProfileInput
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "employer creates an unverified profile keyed by own ID", caller: employer, in: input,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), domain.CompanyProfile{
					EmployerID: employerID, CompanyName: "Druk Tech", Description: "Software", Website: "https://druk.bt", Location: "Thimphu",
					VerificationStatus: domain.VerificationUnverified, CreatedAt: now, UpdatedAt: now,
				}).DoAndReturn(func(_ context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) { return p, nil })
			},
		},
		{name: "applicant is denied", caller: appl, in: input, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin is denied", caller: admin, in: input, wantErr: domain.ErrEmployerRoleRequired},
		{name: "role is checked before input", caller: appl, in: app.ProfileInput{}, wantErr: domain.ErrEmployerRoleRequired},
		{name: "empty company name is rejected", caller: employer, in: app.ProfileInput{CompanyName: "  "}, wantErr: domain.ErrInvalidCompanyName},
		{name: "bad website is rejected", caller: employer, in: app.ProfileInput{CompanyName: "D", Website: "druk.bt"}, wantErr: domain.ErrInvalidWebsite},
		{
			name: "second profile for the same employer already exists", caller: employer, in: input,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.CompanyProfile{}, domain.ErrProfileExists)
			},
			wantErr: domain.ErrProfileExists,
		},
		{
			name: "repository error is passed through", caller: employer, in: input,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.CompanyProfile{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.Create(context.Background(), tt.caller, tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.EmployerID != employerID || got.VerificationStatus != domain.VerificationUnverified {
				t.Errorf("got %+v, want own unverified profile", got)
			}
		})
	}
}

func TestProfiles_GetMine(t *testing.T) {
	for _, tt := range []struct {
		name       string
		caller     domain.Caller
		setup      func(d deps)
		wantReason string
		wantErr    error
	}{
		{
			name: "employer gets own profile with rejection reason", caller: employer,
			setup:      func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil) },
			wantReason: reason,
		},
		{name: "applicant is denied", caller: appl, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin is denied", caller: admin, wantErr: domain.ErrEmployerRoleRequired},
		{
			name: "no profile yet", caller: employer,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.GetMine(context.Background(), tt.caller)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.EmployerID != employerID || got.RejectionReason != tt.wantReason {
				t.Fatalf("got %+v,%v, want own profile with reason %q", got, err, tt.wantReason)
			}
		})
	}
}

func TestProfiles_Update(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		in      app.ProfileInput
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "owner replaces fields and keeps verification", caller: employer,
			in: app.ProfileInput{CompanyName: "Druk Tech Pvt", Location: "Paro"},
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Update(gomock.Any(), domain.CompanyProfile{
					EmployerID: employerID, CompanyName: "Druk Tech Pvt", Location: "Paro",
					VerificationStatus: domain.VerificationRejected, RejectionReason: reason, CreatedAt: created, UpdatedAt: now,
				}).DoAndReturn(func(_ context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) { return p, nil })
			},
		},
		{
			// The request has no employer_id, so another employer can only ever
			// reach their own row. This checks the key is the caller's ID.
			name: "another employer only reaches their own profile", caller: other, in: input,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), otherID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
		{name: "applicant is denied", caller: appl, in: input, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin is denied", caller: admin, in: input, wantErr: domain.ErrEmployerRoleRequired},
		{name: "empty company name is rejected", caller: employer, in: app.ProfileInput{}, wantErr: domain.ErrInvalidCompanyName},
		{
			name: "no profile to update", caller: employer, in: input,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
		{
			name: "profile deleted between read and write", caller: employer, in: input,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.Update(context.Background(), tt.caller, tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.CompanyName != "Druk Tech Pvt" || got.UpdatedAt != now {
				t.Fatalf("got %+v,%v, want updated profile", got, err)
			}
		})
	}
}

func TestProfiles_Get(t *testing.T) {
	for _, tt := range []struct {
		name       string
		caller     domain.Caller
		employerID string
		setup      func(d deps)
		wantReason string
		wantErr    error
	}{
		{
			name: "owner sees rejection reason", caller: employer, employerID: employerID,
			setup:      func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil) },
			wantReason: reason,
		},
		{
			name: "admin sees rejection reason", caller: admin, employerID: employerID,
			setup:      func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil) },
			wantReason: reason,
		},
		{
			name: "applicant gets profile without reason", caller: appl, employerID: employerID,
			setup: func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil) },
		},
		{
			name: "other employer gets profile without reason", caller: other, employerID: employerID,
			setup: func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(stored(), nil) },
		},
		{
			name: "uppercase ID is normalized", caller: appl, employerID: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa").Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
		{name: "ID that is not a UUID", caller: appl, employerID: "abc", wantErr: domain.ErrInvalidEmployerID},
		{name: "empty ID", caller: appl, employerID: "", wantErr: domain.ErrInvalidEmployerID},
		{
			name: "unknown employer", caller: appl, employerID: otherID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), otherID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
			wantErr: domain.ErrProfileNotFound,
		},
		{
			name: "repository error is passed through", caller: appl, employerID: employerID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(domain.CompanyProfile{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newProfiles(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := svc.Get(context.Background(), tt.caller, tt.employerID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.EmployerID != employerID || got.RejectionReason != tt.wantReason {
				t.Fatalf("got %+v,%v, want profile with reason %q", got, err, tt.wantReason)
			}
		})
	}
}
