//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func startAlertsDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("job_alert_test"), pgcontainer.WithUsername("job_alert_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func syntheticAlert(owner, location string) domain.Alert {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.Alert{ID: uuid.NewString(), ApplicantID: owner, CategoryIDs: []string{}, TagIDs: []string{}, Location: location, Names: domain.Names{Categories: map[string]string{}, Tags: map[string]string{}}, CreatedAt: now, UpdatedAt: now}
}
func TestAlerts_PersistenceOwnershipPaginationAndDeletion(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	owner := uuid.NewString()
	other := uuid.NewString()
	a := syntheticAlert(owner, "Thimphu")
	a.CategoryIDs = []string{uuid.NewString()}
	a.Names.Categories[a.CategoryIDs[0]] = "Engineering"
	_, err := repo.Create(ctx, a)
	require.NoError(t, err)
	duplicate := a
	duplicate.ID = uuid.NewString()
	_, err = repo.Create(ctx, duplicate)
	require.ErrorIs(t, err, domain.ErrExists)
	_, err = repo.Create(ctx, syntheticAlert(other, "Thimphu"))
	require.NoError(t, err)
	b := syntheticAlert(owner, "Paro")
	b.CreatedAt = a.CreatedAt.Add(time.Second)
	_, err = repo.Create(ctx, b)
	require.NoError(t, err)
	items, total, err := repo.List(ctx, owner, domain.Page{Size: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, []domain.Alert{a}, items)
	items, total, err = repo.List(ctx, owner, domain.Page{Size: 1, Offset: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, []domain.Alert{b}, items)
	items, total, err = repo.List(ctx, owner, domain.Page{Size: 20, Offset: 9223372036854775807})
	require.NoError(t, err)
	require.Empty(t, items)
	require.EqualValues(t, 2, total)
	require.ErrorIs(t, repo.Delete(ctx, a.ID, other), domain.ErrPermissionDenied)
	require.NoError(t, repo.Delete(ctx, a.ID, owner))
	require.ErrorIs(t, repo.Delete(ctx, a.ID, owner), domain.ErrNotFound)
	_, err = repo.Create(ctx, duplicate)
	require.NoError(t, err)
	pool.Close()
	_, err = repo.Create(ctx, a)
	require.Error(t, err)
	_, _, err = repo.List(ctx, owner, domain.Page{Size: 20})
	require.Error(t, err)
	require.Error(t, repo.Delete(ctx, a.ID, owner))
}
func TestAlerts_ConcurrentLimitAndDuplicates(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	owner := uuid.NewString()
	for i := range 9 {
		_, err := repo.Create(ctx, syntheticAlert(owner, fmt.Sprint(i)))
		require.NoError(t, err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := range 2 {
		go func(i int) {
			<-start
			_, err := repo.Create(ctx, syntheticAlert(owner, fmt.Sprint(i+9)))
			results <- err
		}(i)
	}
	close(start)
	successes, limits := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrLimit) {
			limits++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, limits)
	_, err := repo.Create(ctx, syntheticAlert(owner, "0"))
	require.ErrorIs(t, err, domain.ErrExists)
	owner = uuid.NewString()
	start = make(chan struct{})
	for range 2 {
		go func() { <-start; _, err := repo.Create(ctx, syntheticAlert(owner, "same")); results <- err }()
	}
	close(start)
	successes, duplicates := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrExists) {
			duplicates++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, duplicates)
}
func TestAlerts_MigrationFilterAndLengthConstraints(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	_, err := repo.Create(ctx, syntheticAlert(uuid.NewString(), strings.Repeat("ཀ", 100)))
	require.NoError(t, err)
	for _, tc := range []struct {
		name, location   string
		categories, tags []string
	}{
		{"empty", "", []string{}, []string{}}, {"long location", strings.Repeat("ཀ", 101), []string{}, []string{}}, {"too many categories", "x", make([]string, 21), []string{}}, {"too many tags", "x", []string{}, make([]string, 21)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := syntheticAlert(uuid.NewString(), tc.location)
			a.CategoryIDs = tc.categories
			a.TagIDs = tc.tags
			_, err := repo.Create(ctx, a)
			require.Error(t, err)
		})
	}
}
