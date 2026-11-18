//go:build integration

package postgres_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// startEmployerDB, newProfile, employerID and unknownID are in profiles_integration_test.go.

func dbLogo(signature []byte, n int) []byte {
	b := make([]byte, n)
	copy(b, signature)
	return b
}

var (
	dbJPEG = []byte{0xff, 0xd8, 0xff}
	dbPNG  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

func TestProfiles_PutLogo_StoresReplacesAndKeepsVerification(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	at := time.Date(2026, 11, 18, 10, 0, 0, 123456000, time.UTC)
	later, latest := at.Add(time.Hour), at.Add(2*time.Hour)
	if _, err := repo.Create(ctx, newProfile(employerID, "Druk Tech", at)); err != nil {
		t.Fatal(err)
	}
	// Stands in for the verification consumer.
	if _, err := pool.Exec(ctx,
		`UPDATE company_profiles SET verification_status = 'rejected', rejection_reason = 'missing licence' WHERE employer_id = $1`,
		employerID); err != nil {
		t.Fatal(err)
	}

	before, err := repo.GetByEmployerID(ctx, employerID)
	if err != nil || before.HasLogo {
		t.Fatalf("before upload: %+v,%v, want has_logo false", before, err)
	}
	if _, err := repo.GetLogo(ctx, employerID); !errors.Is(err, domain.ErrLogoNotFound) {
		t.Fatalf("err=%v, want ErrLogoNotFound before upload", err)
	}

	png := domain.Logo{ContentType: "image/png", Content: dbLogo(dbPNG, 1000)}
	updated, err := repo.PutLogo(ctx, employerID, png, later)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasLogo || updated.VerificationStatus != domain.VerificationRejected ||
		updated.RejectionReason != "missing licence" || !updated.UpdatedAt.Equal(later) || !updated.CreatedAt.Equal(at) {
		t.Fatalf("after upload %+v, want has_logo, unchanged verification and created_at, updated_at %v", updated, later)
	}
	got, err := repo.GetLogo(ctx, employerID)
	if err != nil || got.ContentType != "image/png" || !bytes.Equal(got.Content, png.Content) {
		t.Fatalf("get logo %q with %d bytes,%v, want the PNG", got.ContentType, len(got.Content), err)
	}

	jpeg := domain.Logo{ContentType: "image/jpeg", Content: dbLogo(dbJPEG, 500)}
	if _, err := repo.PutLogo(ctx, employerID, jpeg, latest); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetLogo(ctx, employerID)
	if err != nil || got.ContentType != "image/jpeg" || !bytes.Equal(got.Content, jpeg.Content) {
		t.Fatalf("after replace %q with %d bytes,%v, want the JPEG", got.ContentType, len(got.Content), err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company_logos WHERE employer_id = $1`, employerID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("logo rows=%d,%v, want 1", rows, err)
	}
	after, err := repo.GetByEmployerID(ctx, employerID)
	if err != nil || !after.HasLogo || after.VerificationStatus != domain.VerificationRejected || !after.UpdatedAt.Equal(latest) {
		t.Fatalf("profile after replace %+v,%v", after, err)
	}

	t.Run("profile update keeps the logo", func(t *testing.T) {
		u, err := repo.Update(ctx, after.WithFields(domain.ProfileFields{CompanyName: "Druk Tech Pvt"}, latest.Add(time.Hour)))
		if err != nil || !u.HasLogo {
			t.Fatalf("update %+v,%v, want has_logo true", u, err)
		}
	})
}

func TestProfiles_PutLogo_UnknownEmployer(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	logo := domain.Logo{ContentType: "image/jpeg", Content: dbLogo(dbJPEG, 10)}
	if _, err := repo.PutLogo(ctx, unknownID, logo, time.Now().UTC()); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("err=%v, want ErrProfileNotFound", err)
	}
	if _, err := repo.GetLogo(ctx, unknownID); !errors.Is(err, domain.ErrLogoNotFound) {
		t.Fatalf("err=%v, want ErrLogoNotFound", err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM company_logos`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("logo rows=%d,%v, want 0", rows, err)
	}
}

// The database limits back up the domain limits (JT-102). Inserts go straight
// to the table so the domain checks are bypassed.
func TestProfiles_Logo_DatabaseLimits(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	at := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name        string
		id          string
		contentType string
		content     []byte
		wantErr     bool
	}{
		{"0 bytes", "b0000000-0000-4000-8000-000000000001", "image/png", []byte{}, true},
		{"1 byte", "b0000000-0000-4000-8000-000000000002", "image/jpeg", []byte{0xff}, true},
		{"smallest JPEG signature", "b0000000-0000-4000-8000-000000000003", "image/jpeg", dbLogo(dbJPEG, 3), false},
		{"1,000,000 bytes", "b0000000-0000-4000-8000-000000000004", "image/png", dbLogo(dbPNG, 1_000_000), false},
		{"1,000,001 bytes", "b0000000-0000-4000-8000-000000000005", "image/png", dbLogo(dbPNG, 1_000_001), true},
		{"GIF type", "b0000000-0000-4000-8000-000000000006", "image/gif", dbLogo(dbPNG, 10), true},
		{"PNG declared, JPEG bytes", "b0000000-0000-4000-8000-000000000007", "image/png", dbLogo(dbJPEG, 10), true},
		{"JPEG declared, PNG bytes", "b0000000-0000-4000-8000-000000000008", "image/jpeg", dbLogo(dbPNG, 10), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := repo.Create(ctx, newProfile(tt.id, "D", at)); err != nil {
				t.Fatal(err)
			}
			_, err := pool.Exec(ctx,
				`INSERT INTO company_logos (employer_id, content_type, content, updated_at) VALUES ($1, $2, $3, $4)`,
				tt.id, tt.contentType, tt.content, at)
			if tt.wantErr && err == nil {
				t.Fatal("expected the database to reject the insert")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	t.Run("logo without a profile", func(t *testing.T) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO company_logos (employer_id, content_type, content, updated_at) VALUES ($1, 'image/jpeg', $2, $3)`,
			unknownID, dbLogo(dbJPEG, 10), at); err == nil {
			t.Fatal("expected the foreign key to reject the insert")
		}
	})
}
