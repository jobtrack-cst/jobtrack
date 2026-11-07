//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func startParticipantsDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("offer_test"), pgcontainer.WithUsername("offer_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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
		t.Fatal("start test PostgreSQL failed", err)
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get test connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal("open test pool failed")
	}
	t.Cleanup(pool.Close)
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool
}

func TestParticipants_GetByApplicationID_RealPostgres(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	want := domain.ApplicationParticipants{ApplicationID: "aaaaaaaa-1111-4111-8111-111111111111", ApplicantID: "bbbbbbbb-2222-4222-8222-222222222222", EmployerID: "cccccccc-3333-4333-8333-333333333333"}
	if _, err := pool.Exec(ctx, `INSERT INTO application_participants VALUES ($1,$2,$3)`, want.ApplicationID, want.ApplicantID, want.EmployerID); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewParticipants(pool)
	for _, tc := range []struct {
		name, id string
		want     domain.ApplicationParticipants
		err      error
	}{
		{"existing", want.ApplicationID, want, nil},
		{"uppercase UUID", strings.ToUpper(want.ApplicationID), want, nil},
		{"unknown", "00000000-0000-4000-8000-000000000000", domain.ApplicationParticipants{}, domain.ErrParticipantsNotFound},
		{"invalid", "invalid", domain.ApplicationParticipants{}, domain.ErrInvalidApplicationID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.GetByApplicationID(ctx, tc.id)
			if got != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("got %+v, %v; want %+v, %v", got, err, tc.want, tc.err)
			}
		})
	}
	t.Run("database failure", func(t *testing.T) {
		pool.Close()
		got, err := repo.GetByApplicationID(ctx, want.ApplicationID)
		if err == nil || errors.Is(err, domain.ErrParticipantsNotFound) || errors.Is(err, domain.ErrInvalidApplicationID) || got != (domain.ApplicationParticipants{}) {
			t.Fatalf("got %+v, %v", got, err)
		}
		if err.Error() != "get application participants failed" {
			t.Fatalf("unexpected database error: %v", err)
		}
	})
}

func TestParticipants_MigrationConstraints(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	id := "11111111-1111-4111-8111-111111111111"
	if _, err := pool.Exec(ctx, `INSERT INTO application_participants VALUES ($1,$1,$1)`, id); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql, code string }{
		{"primary key", `INSERT INTO application_participants VALUES ($1,$1,$1)`, "23505"},
		{"application required", `INSERT INTO application_participants VALUES (NULL,$1,$1)`, "23502"},
		{"applicant required", `INSERT INTO application_participants VALUES ('22222222-2222-4222-8222-222222222222',NULL,$1)`, "23502"},
		{"employer required", `INSERT INTO application_participants VALUES ('22222222-2222-4222-8222-222222222222',$1,NULL)`, "23502"},
		{"application UUID", `INSERT INTO application_participants VALUES ('invalid',$1,$1)`, "22P02"},
		{"applicant UUID", `INSERT INTO application_participants VALUES ($1,'invalid',$1)`, "22P02"},
		{"employer UUID", `INSERT INTO application_participants VALUES ($1,$1,'invalid')`, "22P02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("got %v, want SQLSTATE %s", err, tc.code)
			}
		})
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'application_participants'::regclass AND contype = 'f'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("foreign keys=%d, error=%v", count, err)
	}
}
