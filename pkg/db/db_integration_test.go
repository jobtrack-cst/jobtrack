//go:build integration

package db_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrate_AppliesSQLAndReleasesResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("helper_test"), postgres.WithUsername("helper_test"),
		postgres.WithPassword("synthetic-test-password"), postgres.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate test database failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start PostgreSQL test container failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable", "pool_max_conns=1")
	if err != nil {
		t.Fatal("get test database URL failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	writeMigration := func(t *testing.T, dir, name, sql string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0600); err != nil {
			t.Fatal(err)
		}
	}
	assertPoolUsable := func(t *testing.T) {
		t.Helper()
		if pool.Stat().AcquiredConns() != 0 {
			t.Fatal("migration helper leaked an acquired connection")
		}
		pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
		defer pingCancel()
		if err := pool.Ping(pingCtx); err != nil {
			t.Fatal("migration helper left caller's pool unusable")
		}
	}

	directory := t.TempDir()
	writeMigration(t, directory, "00001_probe.sql", "-- +goose Up\nCREATE TABLE helper_probe (id integer PRIMARY KEY);\nINSERT INTO helper_probe VALUES (1);\n-- +goose Down\nDROP TABLE helper_probe;\n")
	for range 2 {
		if err := db.Migrate(ctx, pool, directory); err != nil {
			t.Fatal(err)
		}
		assertPoolUsable(t)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM helper_probe").Scan(&count); err != nil || count != 1 {
		t.Fatal("migration was not applied exactly once")
	}

	t.Run("invalid SQL rolls back and releases connections", func(t *testing.T) {
		badDir := t.TempDir()
		writeMigration(t, badDir, "00002_bad.sql", "-- +goose Up\nCREATE TABLE rollback_probe (id integer);\nSELECT 'sensitive-sql-value'::integer;\n-- +goose Down\nDROP TABLE rollback_probe;\n")
		err := db.Migrate(ctx, pool, badDir)
		if err == nil {
			t.Fatal("expected SQL migration error")
		}
		if strings.Contains(err.Error(), "sensitive-sql-value") || strings.Contains(err.Error(), "synthetic-test-password") {
			t.Fatal("migration error exposed sensitive values")
		}
		assertPoolUsable(t)
		var rolledBack bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('rollback_probe') IS NULL").Scan(&rolledBack); err != nil || !rolledBack {
			t.Fatal("failed migration was not rolled back")
		}
	})

	t.Run("provider loading error preserves pool", func(t *testing.T) {
		badDir := t.TempDir()
		for _, name := range []string{"00002_first.sql", "00002_duplicate.sql"} {
			writeMigration(t, badDir, name, "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n")
		}
		if err := db.Migrate(ctx, pool, badDir); err == nil {
			t.Fatal("expected duplicate migration version error")
		}
		assertPoolUsable(t)
	})

	t.Run("cancellation releases resources", func(t *testing.T) {
		slowDir := t.TempDir()
		writeMigration(t, slowDir, "00002_slow.sql", "-- +goose Up\nSELECT pg_sleep(30);\n-- +goose Down\nSELECT 1;\n")
		shortCtx, shortCancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer shortCancel()
		if err := db.Migrate(shortCtx, pool, slowDir); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected migration deadline, got %v", err)
		}
		assertPoolUsable(t)
	})
}
