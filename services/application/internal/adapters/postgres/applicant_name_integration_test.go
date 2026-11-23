//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	"github.com/pressly/goose/v3"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var migrationsDir = filepath.Join("..", "..", "..", "migrations")

// startApplicantNamePostgres starts an empty synthetic PostgreSQL for one test.
func startApplicantNamePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("application_test"), pgcontainer.WithUsername("application_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanup); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func pgCode(err error) string {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Code
	}
	return ""
}

func newApplication(name string) domain.Application {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.Application{ID: uuid.NewString(), ApplicantID: uuid.NewString(), ApplicantName: name, EmployerID: uuid.NewString(), JobID: uuid.NewString(), ResumeID: uuid.NewString(), CreatedAt: now, UpdatedAt: now}
}

// Applications stored before JT-114 keep an empty name; the column then
// enforces NOT NULL and 0 to 200 characters, and the migration reverses cleanly.
func TestMigration_ApplicantNameColumn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startApplicantNamePostgres(t, ctx)

	// Apply only the migrations before JT-114 and store a legacy row.
	before := t.TempDir()
	for _, file := range []string{"00001_initial.sql", "00002_applications.sql", "00003_outbox.sql"} {
		content, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(before, file), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx, pool, before); err != nil {
		t.Fatal(err)
	}
	legacy := newApplication("")
	if _, err := pool.Exec(ctx, "INSERT INTO applications (application_id, applicant_id, job_id, resume_id, employer_id, withdrawn, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,false,$6,$6)",
		legacy.ID, legacy.ApplicantID, legacy.JobID, legacy.ResumeID, legacy.EmployerID, legacy.CreatedAt); err != nil {
		t.Fatal(err)
	}

	// The full directory adds JT-114; applying it twice changes nothing.
	for range 2 {
		if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewRepository(pool)
	got, err := repo.Get(ctx, legacy.ID)
	if err != nil || got != legacy {
		t.Fatalf("legacy row changed: %+v, %v", got, err)
	}

	for _, tt := range []struct {
		name, value string
		code        string // "" stores the row; otherwise the PostgreSQL error code
	}{
		{"empty", "", ""},
		{"one character", "A", ""},
		{"200 characters", strings.Repeat("a", 200), ""},
		{"200 multibyte characters", strings.Repeat("ཀ", 200), ""},
		{"201 characters", strings.Repeat("a", 201), "23514"},
		{"201 multibyte characters", strings.Repeat("ཀ", 201), "23514"},
	} {
		t.Run("check "+tt.name, func(t *testing.T) {
			a := newApplication(tt.value)
			saved, err := repo.Create(ctx, a)
			if got := pgCode(err); got != tt.code {
				t.Fatalf("code=%q (%v), want %q", got, err, tt.code)
			}
			if tt.code == "" && saved.ApplicantName != tt.value {
				t.Fatal("name not stored exactly")
			}
			if tt.code != "" {
				var count int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM applications WHERE application_id=$1", a.ID).Scan(&count); err != nil || count != 0 {
					t.Fatalf("rejected row stored: count=%d err=%v", count, err)
				}
			}
		})
	}
	t.Run("check NULL", func(t *testing.T) {
		a := newApplication("")
		_, err := pool.Exec(ctx, "INSERT INTO applications (application_id, applicant_id, job_id, resume_id, employer_id, withdrawn, created_at, updated_at, applicant_name) VALUES ($1,$2,$3,$4,$5,false,$6,$6,NULL)",
			a.ID, a.ApplicantID, a.JobID, a.ResumeID, a.EmployerID, a.CreatedAt)
		if pgCode(err) != "23502" {
			t.Fatalf("NULL name: %v, want not_null_violation", err)
		}
	})

	t.Run("down and up again", func(t *testing.T) {
		sqlDB := stdlib.OpenDBFromPool(pool)
		defer sqlDB.Close()
		if err := goose.SetDialect("postgres"); err != nil {
			t.Fatal(err)
		}
		if err := goose.DownContext(ctx, sqlDB, migrationsDir); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='applications' AND column_name='applicant_name')").Scan(&exists); err != nil || exists {
			t.Fatalf("column still present after down: %v", err)
		}
		if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.Get(ctx, legacy.ID); err != nil || got.ApplicantName != "" {
			t.Fatalf("legacy row after up: %+v, %v", got, err)
		}
	})
}

// Every repository read returns the stored name: Create, Get, List and Withdraw.
func TestRepository_ApplicantNameRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startApplicantNamePostgres(t, ctx)
	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewRepository(pool)

	a := newApplication("Pema Dorji")
	legacy := newApplication("")
	legacy.EmployerID = a.EmployerID
	legacy.CreatedAt = a.CreatedAt.Add(-time.Minute)
	legacy.UpdatedAt = legacy.CreatedAt
	for _, x := range []domain.Application{a, legacy} {
		saved, err := repo.Create(ctx, x)
		if err != nil || saved != x {
			t.Fatalf("create: %+v, %v", saved, err)
		}
	}
	got, err := repo.Get(ctx, a.ID)
	if err != nil || got.ApplicantName != "Pema Dorji" {
		t.Fatalf("get: %+v, %v", got, err)
	}
	rows, total, err := repo.List(ctx, ports.Filter{EmployerID: a.EmployerID, Limit: 20})
	if err != nil || total != 2 || len(rows) != 2 || rows[0].ApplicantName != "Pema Dorji" || rows[1].ApplicantName != "" {
		t.Fatalf("list: %+v, total=%d, %v", rows, total, err)
	}
	withdrawn, err := repo.Withdraw(ctx, a.ID, a.ApplicantID)
	if err != nil || !withdrawn.Withdrawn || withdrawn.ApplicantName != "Pema Dorji" {
		t.Fatalf("withdraw: %+v, %v", withdrawn, err)
	}
}

// With the real repository, a submission whose name cannot be read stores
// nothing and returns UNAVAILABLE; a readable name is stored.
func TestSubmit_ApplicantNameFailsClosedWithRealRepository(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startApplicantNamePostgres(t, ctx)
	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewRepository(pool)
	for _, tt := range []struct {
		name    string
		user    string
		err     error
		want    codes.Code
		applied int
	}{
		{"auth unavailable", "", status.Error(codes.Unavailable, "synthetic"), codes.Unavailable, 0},
		{"invalid name", strings.Repeat("a", 201), nil, codes.Unavailable, 0},
		{"name stored", "Pema Dorji", nil, codes.OK, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			jobs, resumes, users := mocks.NewMockJobPosting(ctrl), mocks.NewMockResume(ctrl), mocks.NewMockUsers(ctrl)
			applicant, employer, job, resume := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			requestCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("x-user-id", applicant, "x-user-role", "applicant"))
			jobs.EXPECT().GetJob(requestCtx, job).Return(domain.Job{ID: job, EmployerID: employer, Published: true, Deadline: time.Now().Add(time.Hour)}, nil)
			resumes.EXPECT().GetResume(requestCtx, resume).Return(domain.Resume{ID: resume, ApplicantID: applicant, Validated: true}, nil)
			users.EXPECT().GetUserName(requestCtx, applicant).Return(tt.user, tt.err)
			a, err := app.NewService(repo, jobs, resumes, users).Submit(requestCtx, job, resume)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v want=%v (%v)", status.Code(err), tt.want, err)
			}
			var count, events int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM applications WHERE applicant_id=$1", applicant).Scan(&count); err != nil {
				t.Fatal(err)
			}
			// No outbox event without a stored application.
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE payload LIKE '%' || $1 || '%'", applicant).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if count != tt.applied || events != tt.applied {
				t.Fatalf("rows=%d events=%d, want %d", count, events, tt.applied)
			}
			if tt.applied == 1 {
				stored, err := repo.Get(ctx, a.ID)
				if err != nil || stored.ApplicantName != tt.user {
					t.Fatalf("stored: %+v, %v", stored, err)
				}
			}
		})
	}
}
