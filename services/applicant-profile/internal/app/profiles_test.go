package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports/mocks"
)

const (
	applicantID = "3f2b8c1e-1111-4222-8333-444455556666"
	otherID     = "9a8b7c6d-1111-4222-8333-444455556666"
)

var (
	now       = time.Date(2026, 11, 12, 12, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	nowUTC    = now.UTC()
	applicant = domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}
	employer  = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	admin     = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	errDB     = errors.New("database down")
)

func newFixture(t *testing.T) (*mocks.MockProfileRepository, *app.Profiles) {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	repo := mocks.NewMockProfileRepository(ctrl)
	return repo, app.NewProfiles(repo, clock)
}

func input() domain.ProfileInput {
	return domain.ProfileInput{
		Summary:   " Backend developer ",
		Education: []domain.Education{{Institution: "CST", Qualification: "BE", StartYear: 2023}},
		Skills:    []string{"Go", " SQL "},
	}
}

func content() domain.ProfileContent {
	return domain.ProfileContent{
		Summary:   "Backend developer",
		Education: []domain.Education{{Institution: "CST", Qualification: "BE", StartYear: 2023}},
		Skills:    []string{"Go", "SQL"},
	}
}

func stored() domain.Profile {
	return domain.Profile{ApplicantID: applicantID, ProfileContent: content(), CreatedAt: nowUTC.Add(-time.Hour), UpdatedAt: nowUTC}
}

// Expectations not set up in a case fail the test if called, so the error
// cases also prove that nothing is stored or read.
func TestProfiles_Create(t *testing.T) {
	t.Run("stores the caller's trimmed profile with UTC timestamps", func(t *testing.T) {
		repo, svc := newFixture(t)
		want := domain.Profile{ApplicantID: applicantID, ProfileContent: content(), CreatedAt: nowUTC, UpdatedAt: nowUTC}
		repo.EXPECT().Create(gomock.Any(), want).Return(nil)
		got, err := svc.Create(context.Background(), applicant, input())
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	tests := []struct {
		name    string
		caller  domain.Caller
		in      domain.ProfileInput
		repoErr error
		wantErr error
	}{
		{"employer denied", employer, input(), nil, domain.ErrApplicantRoleRequired},
		{"admin denied", admin, input(), nil, domain.ErrApplicantRoleRequired},
		{"role checked before input", employer, domain.ProfileInput{Skills: []string{""}}, nil, domain.ErrApplicantRoleRequired},
		{"invalid input", applicant, domain.ProfileInput{Skills: []string{"Go", "go"}}, nil, domain.ErrDuplicateSkill},
		{"second profile", applicant, input(), domain.ErrProfileExists, domain.ErrProfileExists},
		{"repository error", applicant, input(), errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.repoErr != nil {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tt.repoErr)
			}
			got, err := svc.Create(context.Background(), tt.caller, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, domain.Profile{}, got)
		})
	}
}

func TestProfiles_Get(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		lookup  bool
		repoErr error
		wantErr error
	}{
		{"applicant gets own profile", applicant, true, nil, nil},
		{"employer denied before lookup", employer, false, nil, domain.ErrApplicantRoleRequired},
		{"admin denied before lookup", admin, false, nil, domain.ErrApplicantRoleRequired},
		{"no profile yet", applicant, true, domain.ErrProfileNotFound, domain.ErrProfileNotFound},
		{"repository error", applicant, true, errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.lookup {
				result := stored()
				if tt.repoErr != nil {
					result = domain.Profile{}
				}
				// The lookup always uses the caller's own ID.
				repo.EXPECT().Get(gomock.Any(), applicantID).Return(result, tt.repoErr)
			}
			got, err := svc.Get(context.Background(), tt.caller)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Profile{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stored(), got)
		})
	}
}

func TestProfiles_Update(t *testing.T) {
	t.Run("replaces the caller's content and sets updated_at", func(t *testing.T) {
		repo, svc := newFixture(t)
		repo.EXPECT().Replace(gomock.Any(), applicantID, content(), nowUTC).Return(stored(), nil)
		got, err := svc.Update(context.Background(), applicant, input())
		require.NoError(t, err)
		assert.Equal(t, stored(), got)
	})
	t.Run("empty input clears every field", func(t *testing.T) {
		repo, svc := newFixture(t)
		empty := domain.ProfileContent{Education: []domain.Education{}, Skills: []string{}}
		repo.EXPECT().Replace(gomock.Any(), applicantID, empty, nowUTC).Return(domain.Profile{ApplicantID: applicantID, ProfileContent: empty}, nil)
		_, err := svc.Update(context.Background(), applicant, domain.ProfileInput{})
		require.NoError(t, err)
	})
	tests := []struct {
		name    string
		caller  domain.Caller
		in      domain.ProfileInput
		repoErr error
		wantErr error
	}{
		{"employer denied", employer, input(), nil, domain.ErrApplicantRoleRequired},
		{"admin denied", admin, input(), nil, domain.ErrApplicantRoleRequired},
		{"invalid input", applicant, domain.ProfileInput{Education: []domain.Education{{Institution: "CST", Qualification: "BE", StartYear: 1949}}}, nil, domain.ErrInvalidStartYear},
		{"no profile yet", applicant, input(), domain.ErrProfileNotFound, domain.ErrProfileNotFound},
		{"repository error", applicant, input(), errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.repoErr != nil {
				repo.EXPECT().Replace(gomock.Any(), applicantID, gomock.Any(), gomock.Any()).Return(domain.Profile{}, tt.repoErr)
			}
			got, err := svc.Update(context.Background(), tt.caller, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, domain.Profile{}, got)
		})
	}
}
