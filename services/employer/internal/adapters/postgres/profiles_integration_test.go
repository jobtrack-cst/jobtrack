//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	unknownID  = "00000000-0000-4000-8000-000000000000"
)

// startEmployerDB follows the auth pattern (JT-30): a real Postgres with all
// migrations applied, run twice to prove they are safe on every startup.
func startEmployerDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("employer_test"), pgcontainer.WithUsername("employer_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get test connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool
}

func newProfile(id, name string, at time.Time) domain.CompanyProfile {
	return domain.NewCompanyProfile(id, domain.ProfileFields{
		CompanyName: name, Description: "Software", Website: "https://druk.bt", Location: "Thimphu",
	}, at)
}

func TestProfiles_CreateAndGet(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	// Microsecond precision, because Postgres stores microseconds.
	at := time.Date(2026, 11, 7, 10, 0, 0, 123456000, time.UTC)

	created, err := repo.Create(ctx, newProfile(employerID, "Druk Tech", at))
	if err != nil {
		t.Fatal(err)
	}
	want := newProfile(employerID, "Druk Tech", at)
	if created != want {
		t.Fatalf("created %+v, want %+v", created, want)
	}
	if created.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location=%v, want UTC", created.CreatedAt.Location())
	}

	got, err := repo.GetByEmployerID(ctx, employerID)
	if err != nil || got != want {
		t.Fatalf("get %+v,%v, want %+v,nil", got, err, want)
	}
	if got.VerificationStatus != domain.VerificationUnverified {
		t.Fatalf("status=%v, want unverified by default", got.VerificationStatus)
	}

	t.Run("second profile for the same employer", func(t *testing.T) {
		if _, err := repo.Create(ctx, newProfile(employerID, "Another", at)); !errors.Is(err, domain.ErrProfileExists) {
			t.Fatalf("err=%v, want ErrProfileExists", err)
		}
	})
	t.Run("unknown employer", func(t *testing.T) {
		if _, err := repo.GetByEmployerID(ctx, unknownID); !errors.Is(err, domain.ErrProfileNotFound) {
			t.Fatalf("err=%v, want ErrProfileNotFound", err)
		}
	})
}

func TestProfiles_Update_KeepsVerificationAndCreatedAt(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	later := at.Add(time.Hour)
	if _, err := repo.Create(ctx, newProfile(employerID, "Druk Tech", at)); err != nil {
		t.Fatal(err)
	}
	// Stands in for the Sprint 2 employer.verification_rejected consumer.
	if _, err := pool.Exec(ctx,
		`UPDATE company_profiles SET verification_status = 'rejected', rejection_reason = 'missing licence' WHERE employer_id = $1`,
		employerID); err != nil {
		t.Fatal(err)
	}

	// The caller's copy is stale (still unverified); Update must not write it back.
	stale := newProfile(employerID, "Druk Tech", at).WithFields(domain.ProfileFields{CompanyName: "Druk Tech Pvt", Location: "Paro"}, later)
	updated, err := repo.Update(ctx, stale)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.CompanyProfile{
		EmployerID: employerID, CompanyName: "Druk Tech Pvt", Location: "Paro",
		VerificationStatus: domain.VerificationRejected, RejectionReason: "missing licence",
		CreatedAt: at, UpdatedAt: later,
	}
	if updated != want {
		t.Fatalf("updated %+v, want %+v", updated, want)
	}

	t.Run("unknown employer", func(t *testing.T) {
		if _, err := repo.Update(ctx, newProfile(unknownID, "Nobody", later)); !errors.Is(err, domain.ErrProfileNotFound) {
			t.Fatalf("err=%v, want ErrProfileNotFound", err)
		}
	})
}

// The database limits back up the domain limits, counted in characters.
func TestProfiles_DatabaseLimits(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	repo := postgres.NewProfiles(pool)
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name    string
		id      string
		profile func(id string) domain.CompanyProfile
		wantErr bool
	}{
		{"150-character multibyte company name", "a0000000-0000-4000-8000-000000000001",
			func(id string) domain.CompanyProfile { return newProfile(id, strings.Repeat("ä", 150), at) }, false},
		{"151-character company name", "a0000000-0000-4000-8000-000000000002",
			func(id string) domain.CompanyProfile { return newProfile(id, strings.Repeat("ä", 151), at) }, true},
		{"empty company name", "a0000000-0000-4000-8000-000000000003",
			func(id string) domain.CompanyProfile { return newProfile(id, "", at) }, true},
		{"2001-character description", "a0000000-0000-4000-8000-000000000004",
			func(id string) domain.CompanyProfile {
				p := newProfile(id, "D", at)
				p.Description = strings.Repeat("d", 2001)
				return p
			}, true},
		{"256-character website", "a0000000-0000-4000-8000-000000000005",
			func(id string) domain.CompanyProfile {
				p := newProfile(id, "D", at)
				p.Website = strings.Repeat("w", 256)
				return p
			}, true},
		{"101-character location", "a0000000-0000-4000-8000-000000000006",
			func(id string) domain.CompanyProfile {
				p := newProfile(id, "D", at)
				p.Location = strings.Repeat("l", 101)
				return p
			}, true},
		{"employer ID that is not a UUID", "employer-1",
			func(id string) domain.CompanyProfile { return newProfile(id, "D", at) }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.Create(ctx, tt.profile(tt.id))
			if tt.wantErr && err == nil {
				t.Fatal("expected the database to reject the insert")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
