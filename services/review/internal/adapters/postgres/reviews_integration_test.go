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
	"github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var base = time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)

// startReviewDB follows the job-posting pattern (JT-38): a real Postgres with
// all migrations applied, run twice to prove they are safe on every startup.
func startReviewDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("review_test"), pgcontainer.WithUsername("review_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

// seedParticipants stands in for the Sprint 2 application.submitted consumer.
func seedParticipants(t *testing.T, ctx context.Context, pool *pgxpool.Pool) domain.ApplicationParticipants {
	t.Helper()
	p := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}
	_, err := pool.Exec(ctx, `INSERT INTO application_participants (application_id, applicant_id, employer_id) VALUES ($1, $2, $3)`,
		p.ApplicationID, p.ApplicantID, p.EmployerID)
	require.NoError(t, err)
	return p
}

func reviewFor(p domain.ApplicationParticipants, score int32, d domain.Decision) domain.Review {
	return domain.Review{ReviewID: uuid.NewString(), ApplicationID: p.ApplicationID, EmployerID: p.EmployerID,
		Score: score, Decision: d, CreatedAt: base, UpdatedAt: base}
}

func TestReviews_Integration(t *testing.T) {
	ctx, pool := startReviewDB(t)
	participants := postgres.NewParticipants(pool)
	reviews := postgres.NewReviews(pool)

	t.Run("participants are read from the projection", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		got, err := participants.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
	})

	t.Run("unknown or unprocessed application is not found", func(t *testing.T) {
		_, err := participants.GetByApplicationID(ctx, uuid.NewString())
		require.ErrorIs(t, err, domain.ErrApplicationNotFound)
	})

	t.Run("review round-trips every field", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		rv := reviewFor(p, 4, domain.DecisionShortlisted)
		rv.CreatedAt, rv.UpdatedAt = base.Add(123*time.Microsecond), base.Add(123*time.Microsecond)
		require.NoError(t, reviews.Create(ctx, rv))
		got, err := reviews.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, rv, got)
		assert.Equal(t, time.UTC, got.CreatedAt.Location())
	})

	t.Run("both decisions and the score boundaries 1 and 5 are stored", func(t *testing.T) {
		for _, tt := range []struct {
			score    int32
			decision domain.Decision
		}{{1, domain.DecisionShortlisted}, {5, domain.DecisionRejected}} {
			p := seedParticipants(t, ctx, pool)
			require.NoError(t, reviews.Create(ctx, reviewFor(p, tt.score, tt.decision)))
			got, err := reviews.GetByApplicationID(ctx, p.ApplicationID)
			require.NoError(t, err)
			assert.Equal(t, tt.score, got.Score)
			assert.Equal(t, tt.decision, got.Decision)
		}
	})

	t.Run("second review is ALREADY_EXISTS and the first is kept", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		first := reviewFor(p, 2, domain.DecisionRejected)
		require.NoError(t, reviews.Create(ctx, first))
		require.ErrorIs(t, reviews.Create(ctx, reviewFor(p, 5, domain.DecisionShortlisted)), domain.ErrReviewExists)
		got, err := reviews.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, first, got)
	})

	t.Run("simultaneous reviews: exactly one is stored", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		errs := make([]error, 5)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = reviews.Create(ctx, reviewFor(p, int32(i%5)+1, domain.DecisionShortlisted))
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			assert.ErrorIs(t, err, domain.ErrReviewExists)
		}
		assert.Equal(t, 1, ok)
	})

	t.Run("no review yet is not found", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		_, err := reviews.GetByApplicationID(ctx, p.ApplicationID)
		require.ErrorIs(t, err, domain.ErrReviewNotFound)
	})

	t.Run("database refuses scores 0 and 6 and unknown applications", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		for _, score := range []int32{0, 6} {
			err := reviews.Create(ctx, reviewFor(p, score, domain.DecisionShortlisted))
			require.Error(t, err)
			assert.Equal(t, "create review failed", err.Error(), "driver details are hidden")
		}
		ghost := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), EmployerID: uuid.NewString()}
		require.Error(t, reviews.Create(ctx, reviewFor(ghost, 3, domain.DecisionRejected)))
		_, err := reviews.GetByApplicationID(ctx, p.ApplicationID)
		require.ErrorIs(t, err, domain.ErrReviewNotFound, "nothing was stored")
	})

	t.Run("invalid decision is refused before the database", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		require.Error(t, reviews.Create(ctx, reviewFor(p, 3, domain.DecisionUnspecified)))
	})
}
