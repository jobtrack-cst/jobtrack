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

func TestListJobSavers_PagedQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := pgcontainer.Run(ctx, "postgres:17-alpine",
		pgcontainer.WithDatabase("saved_jobs_test"),
		pgcontainer.WithUsername("saved_jobs_test"),
		pgcontainer.WithPassword("synthetic-test-password"),
		pgcontainer.BasicWaitStrategies(),
	)
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

	migrationsDir, _ := filepath.Abs("../../../migrations")
	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	repo := postgres.NewSavedJobsRepository(pool)
	jobID := "33333333-3333-4333-8333-333333333333"

	// Insert 25 savers
	for i := 0; i < 25; i++ {
		applicantID := uuidForTest(i)
		if err := repo.SaveJob(ctx, jobID, applicantID); err != nil {
			t.Fatalf("SaveJob %d: %v", i, err)
		}
	}

	// Page 1: offset 0, limit 10
	savers, total, err := repo.ListJobSavers(ctx, jobID, 10, 0)
	if err != nil {
		t.Fatalf("ListJobSavers page 1: %v", err)
	}
	if total != 25 {
		t.Errorf("page 1: expected total 25, got %d", total)
	}
	if len(savers) != 10 {
		t.Errorf("page 1: expected 10 savers, got %d", len(savers))
	}

	// Page 2: offset 10, limit 10
	savers2, total2, err := repo.ListJobSavers(ctx, jobID, 10, 10)
	if err != nil {
		t.Fatalf("ListJobSavers page 2: %v", err)
	}
	if total2 != 25 {
		t.Errorf("page 2: expected total 25, got %d", total2)
	}
	if len(savers2) != 10 {
		t.Errorf("page 2: expected 10 savers, got %d", len(savers2))
	}

	// Page 3: offset 20, limit 10 — should return 5
	savers3, total3, err := repo.ListJobSavers(ctx, jobID, 10, 20)
	if err != nil {
		t.Fatalf("ListJobSavers page 3: %v", err)
	}
	if total3 != 25 {
		t.Errorf("page 3: expected total 25, got %d", total3)
	}
	if len(savers3) != 5 {
		t.Errorf("page 3: expected 5 savers, got %d", len(savers3))
	}

	// No overlap between pages (implied by stable ordering + limit)
	for _, s := range savers {
		for _, s2 := range savers2 {
			if s == s2 {
				t.Errorf("duplicate saver across pages: %s", s)
			}
		}
	}
}

func uuidForTest(i int) string {
	// Generate a deterministic valid UUID for testing
	const base = "00000000-0000-4000-8000-0000000000"
	suffix := ""
	n := i
	for j := 0; j < 2; j++ {
		suffix = string(rune('0'+n%10)) + suffix
		n /= 10
	}
	return base + suffix
}
