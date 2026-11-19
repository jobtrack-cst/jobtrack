package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// newFixture, applicant, employer, admin, nowUTC, stored and errDB are in profiles_test.go.

func jpeg(n int) []byte {
	b := make([]byte, n)
	copy(b, []byte{0xff, 0xd8, 0xff})
	return b
}

func TestProfiles_PutPhoto(t *testing.T) {
	t.Run("stores the caller's photo with the UTC time and returns the profile", func(t *testing.T) {
		repo, svc := newFixture(t)
		want := stored()
		want.HasPhoto = true
		photo := domain.Photo{ContentType: "image/jpeg", Content: jpeg(1000)}
		repo.EXPECT().PutPhoto(gomock.Any(), applicantID, photo, nowUTC).Return(want, nil)
		got, err := svc.PutPhoto(context.Background(), applicant, "image/jpeg", jpeg(1000))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	tests := []struct {
		name        string
		caller      domain.Caller
		contentType string
		content     []byte
		repoErr     error
		wantErr     error
	}{
		{"employer denied", employer, "image/jpeg", jpeg(10), nil, domain.ErrApplicantRoleRequired},
		{"admin denied", admin, "image/jpeg", jpeg(10), nil, domain.ErrApplicantRoleRequired},
		{"role checked before input", employer, "image/gif", nil, nil, domain.ErrApplicantRoleRequired},
		{"0 bytes not stored", applicant, "image/jpeg", nil, nil, domain.ErrInvalidPhotoSize},
		{"1,000,001 bytes not stored", applicant, "image/jpeg", jpeg(1_000_001), nil, domain.ErrInvalidPhotoSize},
		{"wrong type not stored", applicant, "image/gif", jpeg(10), nil, domain.ErrInvalidPhotoType},
		{"wrong signature not stored", applicant, "image/png", jpeg(10), nil, domain.ErrInvalidPhotoContent},
		{"no profile", applicant, "image/jpeg", jpeg(10), domain.ErrProfileNotFound, domain.ErrProfileNotFound},
		{"repository error passed on", applicant, "image/jpeg", jpeg(10), errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.repoErr != nil {
				repo.EXPECT().PutPhoto(gomock.Any(), applicantID, gomock.Any(), nowUTC).Return(domain.Profile{}, tt.repoErr)
			}
			_, err := svc.PutPhoto(context.Background(), tt.caller, tt.contentType, tt.content)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestProfiles_GetPhoto(t *testing.T) {
	t.Run("returns the caller's own photo", func(t *testing.T) {
		repo, svc := newFixture(t)
		photo := domain.Photo{ContentType: "image/png", Content: []byte{1, 2, 3}}
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(photo, nil)
		got, err := svc.GetPhoto(context.Background(), applicant)
		require.NoError(t, err)
		assert.Equal(t, photo, got)
	})
	tests := []struct {
		name    string
		caller  domain.Caller
		repoErr error
		wantErr error
	}{
		{"employer denied", employer, nil, domain.ErrApplicantRoleRequired},
		{"admin denied", admin, nil, domain.ErrApplicantRoleRequired},
		{"no photo", applicant, domain.ErrPhotoNotFound, domain.ErrPhotoNotFound},
		{"repository error passed on", applicant, errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.repoErr != nil {
				repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, tt.repoErr)
			}
			_, err := svc.GetPhoto(context.Background(), tt.caller)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
