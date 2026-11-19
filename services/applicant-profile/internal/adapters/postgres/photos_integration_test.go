//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// startProfileDB, newProfile, content and base are in profiles_integration_test.go.

func dbPhoto(signature []byte, n int) []byte {
	b := make([]byte, n)
	copy(b, signature)
	return b
}

var (
	dbJPEG = []byte{0xff, 0xd8, 0xff}
	dbPNG  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

func TestProfiles_Photo_Integration(t *testing.T) {
	ctx, pool := startProfileDB(t)
	repo := postgres.NewProfiles(pool)
	later, latest := base.Add(time.Hour+123*time.Microsecond), base.Add(2*time.Hour)

	p := newProfile("photo")
	require.NoError(t, repo.Create(ctx, p))

	t.Run("a new profile has no photo", func(t *testing.T) {
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.False(t, got.HasPhoto)
		_, err = repo.GetPhoto(ctx, p.ApplicantID)
		assert.ErrorIs(t, err, domain.ErrPhotoNotFound)
	})

	t.Run("upload without a profile is NOT_FOUND and stores nothing", func(t *testing.T) {
		unknown := uuid.NewString()
		_, err := repo.PutPhoto(ctx, unknown, domain.Photo{ContentType: "image/png", Content: dbPhoto(dbPNG, 10)}, later)
		assert.ErrorIs(t, err, domain.ErrProfileNotFound)
		_, err = repo.GetPhoto(ctx, unknown)
		assert.ErrorIs(t, err, domain.ErrPhotoNotFound)
	})

	png := domain.Photo{ContentType: "image/png", Content: dbPhoto(dbPNG, 1_000_000)}
	t.Run("upload stores the photo and only changes updated_at", func(t *testing.T) {
		updated, err := repo.PutPhoto(ctx, p.ApplicantID, png, later)
		require.NoError(t, err)
		assert.True(t, updated.HasPhoto)
		assert.Equal(t, p.ProfileContent, updated.ProfileContent, "summary, education and skills unchanged")
		assert.True(t, updated.CreatedAt.Equal(base))
		assert.True(t, updated.UpdatedAt.Equal(later))
		assert.Equal(t, time.UTC, updated.UpdatedAt.Location())

		got, err := repo.GetPhoto(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, png.ContentType, got.ContentType)
		assert.Equal(t, png.Content, got.Content, "the 1,000,000 byte boundary is stored unchanged")

		read, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.True(t, read.HasPhoto)
	})

	t.Run("a second upload replaces the photo", func(t *testing.T) {
		jpeg := domain.Photo{ContentType: "image/jpeg", Content: dbPhoto(dbJPEG, 3)}
		_, err := repo.PutPhoto(ctx, p.ApplicantID, jpeg, latest)
		require.NoError(t, err)
		got, err := repo.GetPhoto(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, jpeg, got)
	})

	t.Run("a profile update keeps the photo", func(t *testing.T) {
		replaced, err := repo.Replace(ctx, p.ApplicantID, content("new"), latest.Add(time.Minute))
		require.NoError(t, err)
		assert.True(t, replaced.HasPhoto)
		_, err = repo.GetPhoto(ctx, p.ApplicantID)
		require.NoError(t, err)
	})

	t.Run("each applicant has their own photo", func(t *testing.T) {
		other := newProfile("other")
		require.NoError(t, repo.Create(ctx, other))
		got, err := repo.Get(ctx, other.ApplicantID)
		require.NoError(t, err)
		assert.False(t, got.HasPhoto)
		_, err = repo.GetPhoto(ctx, other.ApplicantID)
		assert.ErrorIs(t, err, domain.ErrPhotoNotFound)
	})
}

// The database enforces the same limits as the domain, even if a caller
// skips domain.NewPhoto. A rejected upload keeps the previous photo.
func TestProfiles_Photo_DatabaseLimits(t *testing.T) {
	ctx, pool := startProfileDB(t)
	repo := postgres.NewProfiles(pool)
	p := newProfile("limits")
	require.NoError(t, repo.Create(ctx, p))
	valid := domain.Photo{ContentType: "image/jpeg", Content: dbPhoto(dbJPEG, 10)}
	_, err := repo.PutPhoto(ctx, p.ApplicantID, valid, base.Add(time.Hour))
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		photo domain.Photo
	}{
		{"0 bytes", domain.Photo{ContentType: "image/jpeg", Content: []byte{}}},
		{"1,000,001 bytes", domain.Photo{ContentType: "image/jpeg", Content: dbPhoto(dbJPEG, 1_000_001)}},
		{"GIF type", domain.Photo{ContentType: "image/gif", Content: dbPhoto(dbJPEG, 10)}},
		{"PNG declared, JPEG bytes", domain.Photo{ContentType: "image/png", Content: dbPhoto(dbJPEG, 10)}},
		{"JPEG declared, PNG bytes", domain.Photo{ContentType: "image/jpeg", Content: dbPhoto(dbPNG, 10)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.PutPhoto(ctx, p.ApplicantID, tt.photo, base.Add(2*time.Hour))
			require.Error(t, err)
			assert.False(t, errors.Is(err, domain.ErrProfileNotFound))
			assert.Equal(t, "put profile photo failed", err.Error(), "driver details are hidden")
			got, err := repo.GetPhoto(ctx, p.ApplicantID)
			require.NoError(t, err)
			assert.Equal(t, valid, got, "the previous photo is kept")
		})
	}
}
