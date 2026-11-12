//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/postgres"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestReadiness_RealPostgresAndInitialMigration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("template_test"), pgcontainer.WithUsername("template_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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
	defer pool.Close()
	readiness := postgres.NewReadiness(pool)
	if err := readiness.Check(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = 1 AND is_applied").Scan(&count); err != nil || count != 1 {
		t.Fatal("initial migration was not recorded exactly once")
	}
	if err := readiness.Check(ctx); err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if err := readiness.Check(ctx); err == nil {
		t.Fatal("closed pool reported ready")
	}
}
