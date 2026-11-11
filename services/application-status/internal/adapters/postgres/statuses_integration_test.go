//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	applicantID = "11111111-1111-4111-8111-111111111111"
	employerID  = "22222222-2222-4222-8222-222222222222"
	unknownID   = "00000000-0000-4000-8000-000000000000"
)

var base = time.Date(2026, 11, 11, 6, 0, 0, 0, time.UTC)

// startStatusDB follows the job-posting pattern (JT-38): a real Postgres with
// all migrations applied, run twice to prove they are safe on every startup.
func startStatusDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("application_status_test"), pgcontainer.WithUsername("application_status_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

// start records a new SUBMITTED application with a fresh ID.
func start(t *testing.T, ctx context.Context, repo *postgres.Statuses, at time.Time) domain.ApplicationStatus {
	t.Helper()
	s, first, err := domain.NewSubmitted(uuid.NewString(), applicantID, employerID, at)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, s, first))
	return s
}

// step applies one domain transition and stores it.
func step(t *testing.T, ctx context.Context, repo *postgres.Statuses, s domain.ApplicationStatus, to domain.Status, at time.Time) domain.ApplicationStatus {
	t.Helper()
	next, entry, err := s.Transition(to, at)
	require.NoError(t, err)
	require.NoError(t, repo.Transition(ctx, next, s.Status, entry))
	return next
}

func TestStatuses_Integration(t *testing.T) {
	ctx, pool := startStatusDB(t)
	repo := postgres.NewStatuses(pool)

	t.Run("create stores SUBMITTED and the first history entry", func(t *testing.T) {
		s := start(t, ctx, repo, base.Add(123*time.Microsecond))
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, s, got)
		assert.Equal(t, time.UTC, got.CreatedAt.Location())

		history, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, []domain.HistoryEntry{{
			ApplicationID: s.ApplicationID, PreviousStatus: domain.StatusUnspecified,
			NewStatus: domain.StatusSubmitted, OccurredAt: s.CreatedAt,
		}}, history)
	})

	t.Run("duplicate create is ALREADY_EXISTS and adds no history", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		dup, first, err := domain.NewSubmitted(s.ApplicationID, applicantID, employerID, base.Add(time.Hour))
		require.NoError(t, err)
		require.ErrorIs(t, repo.Create(ctx, dup, first), domain.ErrApplicationExists)
		_, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, base, got.CreatedAt, "original row unchanged")
	})

	t.Run("unknown application is not found", func(t *testing.T) {
		_, err := repo.Get(ctx, unknownID)
		require.ErrorIs(t, err, domain.ErrApplicationNotFound)
		ghost := domain.ApplicationStatus{ApplicationID: unknownID, Status: domain.StatusUnderReview, UpdatedAt: base}
		entry := domain.HistoryEntry{ApplicationID: unknownID, PreviousStatus: domain.StatusSubmitted, NewStatus: domain.StatusUnderReview, OccurredAt: base}
		require.ErrorIs(t, repo.Transition(ctx, ghost, domain.StatusSubmitted, entry), domain.ErrApplicationNotFound)
	})

	t.Run("full lifecycle is stored with every transition in history, oldest first", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		path := []domain.Status{domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusOffered, domain.StatusAccepted}
		for i, to := range path {
			s = step(t, ctx, repo, s, to, base.Add(time.Duration(i+1)*time.Minute))
		}
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusAccepted, got.Status)
		assert.Equal(t, base.Add(5*time.Minute), got.UpdatedAt)
		assert.Equal(t, base, got.CreatedAt)

		history, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 6, total)
		want := []domain.Status{domain.StatusSubmitted, domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusOffered, domain.StatusAccepted}
		prev := domain.StatusUnspecified
		for i, e := range history {
			assert.Equal(t, prev, e.PreviousStatus)
			assert.Equal(t, want[i], e.NewStatus)
			assert.Equal(t, base.Add(time.Duration(i)*time.Minute), e.OccurredAt)
			prev = e.NewStatus
		}

		page2, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 4, Offset: 4})
		require.NoError(t, err)
		assert.Equal(t, 6, total, "total_count is the full count on every page")
		require.Len(t, page2, 2)
		assert.Equal(t, domain.StatusOffered, page2[0].NewStatus)
		assert.Equal(t, domain.StatusAccepted, page2[1].NewStatus)
	})

	t.Run("entries with the same time keep insertion order", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		s = step(t, ctx, repo, s, domain.StatusUnderReview, base)
		s = step(t, ctx, repo, s, domain.StatusWithdrawn, base)
		history, _, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		require.Len(t, history, 3)
		assert.Equal(t, []domain.Status{domain.StatusSubmitted, domain.StatusUnderReview, domain.StatusWithdrawn},
			[]domain.Status{history[0].NewStatus, history[1].NewStatus, history[2].NewStatus})
	})

	t.Run("stale expected status is a concurrent change and stores nothing", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		moved := step(t, ctx, repo, s, domain.StatusUnderReview, base.Add(time.Minute))
		// A second writer still believes the application is SUBMITTED.
		stale, entry, err := s.Transition(domain.StatusRejected, base.Add(2*time.Minute))
		require.NoError(t, err)
		require.ErrorIs(t, repo.Transition(ctx, stale, domain.StatusSubmitted, entry), domain.ErrStatusChanged)
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, moved, got)
		_, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
	})

	t.Run("simultaneous transitions: exactly one is stored", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		targets := []domain.Status{domain.StatusUnderReview, domain.StatusRejected, domain.StatusWithdrawn, domain.StatusUnderReview}
		errs := make([]error, len(targets))
		var wg sync.WaitGroup
		for i, to := range targets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				next, entry, err := s.Transition(to, base.Add(time.Minute))
				if err != nil {
					errs[i] = err
					return
				}
				errs[i] = repo.Transition(ctx, next, domain.StatusSubmitted, entry)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			assert.ErrorIs(t, err, domain.ErrStatusChanged)
		}
		assert.Equal(t, 1, ok)
		_, total, err := repo.ListHistory(ctx, s.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 2, total, "one first entry and one transition")
	})

	t.Run("database refuses a non-SUBMITTED first entry and unknown statuses", func(t *testing.T) {
		bad, first, err := domain.NewSubmitted(uuid.NewString(), applicantID, employerID, base)
		require.NoError(t, err)
		first.NewStatus = domain.StatusUnderReview
		err = repo.Create(ctx, bad, first)
		require.Error(t, err)
		assert.Equal(t, "create application status failed", err.Error(), "driver details are hidden")
		_, err = repo.Get(ctx, bad.ApplicationID)
		require.ErrorIs(t, err, domain.ErrApplicationNotFound, "the failed create left no status row")

		bad.Status = domain.StatusUnspecified
		require.Error(t, repo.Create(ctx, bad, first))
	})

	t.Run("application with no history gets an empty list", func(t *testing.T) {
		got, total, err := repo.ListHistory(ctx, unknownID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
