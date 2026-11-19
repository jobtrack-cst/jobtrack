//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

const (
	cjJobID         = "33333333-3333-4333-8333-333333333333" // job in job.closed.json
	cjOtherJobID    = "44444444-4444-4444-8444-444444444444"
	cjApplicantA    = "a1111111-1111-4111-8111-111111111111"
	cjApplicantB    = "b2222222-2222-4222-8222-222222222222"
	cjApplicantC    = "c3333333-3333-4333-8333-333333333333"
	cjEventID       = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2" // event in job.closed.json
	cjSecondEventID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb3"
)

var cjClosedAt = time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

func startCloseJobPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine",
		pgcontainer.WithDatabase("saved_jobs_test"), pgcontainer.WithUsername("saved_jobs_test"),
		pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := db.OpenPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")))
	return pool
}

func cjClosure(t *testing.T, eventID, jobID string, closedAt time.Time) domain.JobClosure {
	t.Helper()
	closure, err := domain.NewJobClosure(eventID, jobID, closedAt)
	require.NoError(t, err)
	return closure
}

func cjCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&n))
	return n
}

func cjStoredClosedAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool) time.Time {
	t.Helper()
	var stored time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT closed_at FROM closed_jobs WHERE job_id = $1", cjJobID).Scan(&stored))
	return stored
}

func TestSavedJobsRepository_CloseJob_IntegrationDeletesBookmarksAndRemembersJob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startCloseJobPostgres(t, ctx)
	repo := postgres.NewSavedJobsRepository(pool)
	for _, s := range []struct{ job, applicant string }{
		{cjJobID, cjApplicantA}, {cjJobID, cjApplicantB}, {cjOtherJobID, cjApplicantA},
	} {
		require.NoError(t, repo.SaveJob(ctx, s.job, s.applicant))
	}

	got, err := repo.CloseJob(ctx, cjClosure(t, cjEventID, cjJobID, cjClosedAt))
	require.NoError(t, err)
	require.Equal(t, domain.ClosureOutcome{RemovedBookmarks: 2}, got)

	_, total, err := repo.ListJobSavers(ctx, cjJobID, 20, 0)
	require.NoError(t, err)
	require.Zero(t, total, "every bookmark for the closed job is deleted")
	_, total, err = repo.ListJobSavers(ctx, cjOtherJobID, 20, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total, "other jobs keep their bookmarks")

	require.True(t, cjClosedAt.Equal(cjStoredClosedAt(t, ctx, pool)))
	require.ErrorIs(t, repo.SaveJob(ctx, cjJobID, cjApplicantC), domain.ErrJobClosed)
	require.ErrorIs(t, repo.SaveJob(ctx, strings.ToUpper(cjJobID), cjApplicantC), domain.ErrJobClosed)
}

func TestSavedJobsRepository_CloseJob_IntegrationIgnoresProcessedEventID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startCloseJobPostgres(t, ctx)
	repo := postgres.NewSavedJobsRepository(pool)

	got, err := repo.CloseJob(ctx, cjClosure(t, cjEventID, cjJobID, cjClosedAt))
	require.NoError(t, err)
	require.Equal(t, domain.ClosureOutcome{}, got, "closing a job nobody saved removes nothing")

	// A row written behind the repository's back shows whether a repeat changes anything.
	_, err = pool.Exec(ctx, "INSERT INTO saved_jobs (job_id, applicant_id) VALUES ($1, $2)", cjJobID, cjApplicantA)
	require.NoError(t, err)
	got, err = repo.CloseJob(ctx, cjClosure(t, cjEventID, cjJobID, cjClosedAt.Add(time.Hour)))
	require.NoError(t, err)
	require.Equal(t, domain.ClosureOutcome{Duplicate: true}, got)
	require.Equal(t, 1, cjCount(t, ctx, pool, "SELECT count(*) FROM saved_jobs WHERE job_id = $1", cjJobID))
	require.Equal(t, 1, cjCount(t, ctx, pool, "SELECT count(*) FROM processed_events"))

	// A new event ID for the same job deletes again and keeps the first closed_at.
	got, err = repo.CloseJob(ctx, cjClosure(t, cjSecondEventID, cjJobID, cjClosedAt.Add(time.Hour)))
	require.NoError(t, err)
	require.Equal(t, domain.ClosureOutcome{RemovedBookmarks: 1}, got)
	require.True(t, cjClosedAt.Equal(cjStoredClosedAt(t, ctx, pool)))
	require.Equal(t, 2, cjCount(t, ctx, pool, "SELECT count(*) FROM processed_events"))
}

func TestSavedJobsRepository_CloseJob_IntegrationConcurrentSavesLeaveNoBookmark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := startCloseJobPostgres(t, ctx)
	repo := postgres.NewSavedJobsRepository(pool)

	const savers = 20
	saveErrs := make([]error, savers)
	var closeErr error
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range savers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			saveErrs[i] = repo.SaveJob(ctx, cjJobID, fmt.Sprintf("c0000000-0000-4000-8000-%012d", i))
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, closeErr = repo.CloseJob(ctx, cjClosure(t, cjEventID, cjJobID, cjClosedAt))
	}()
	close(start)
	wg.Wait()

	require.NoError(t, closeErr)
	for i, err := range saveErrs {
		if err != nil {
			require.ErrorIs(t, err, domain.ErrJobClosed, "saver %d", i)
		}
	}
	require.Zero(t, cjCount(t, ctx, pool, "SELECT count(*) FROM saved_jobs WHERE job_id = $1", cjJobID),
		"a save is either deleted by the close or refused after it")
}
