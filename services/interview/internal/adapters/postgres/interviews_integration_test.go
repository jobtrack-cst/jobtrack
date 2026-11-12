//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var base = time.Date(2026, 11, 20, 4, 0, 0, 0, time.UTC)

// startInterviewDB follows the job-posting pattern (JT-38): a real Postgres
// with all migrations applied, run twice to prove they are safe on every startup.
func startInterviewDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("interview_test"), pgcontainer.WithUsername("interview_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func interviewFor(p domain.ApplicationParticipants, at time.Time) domain.Interview {
	return domain.Interview{InterviewID: uuid.NewString(), ApplicationID: p.ApplicationID, EmployerID: p.EmployerID,
		ApplicantID: p.ApplicantID, ScheduledAt: at, Details: "Room 3\nBlock B", CreatedAt: base.Add(-time.Hour), UpdatedAt: base.Add(-time.Hour)}
}

func TestInterviews_Integration(t *testing.T) {
	ctx, pool := startInterviewDB(t)
	participants := postgres.NewParticipants(pool)
	interviews := postgres.NewInterviews(pool)

	t.Run("participants are read from the projection", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		got, err := participants.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
		_, err = participants.GetByApplicationID(ctx, uuid.NewString())
		require.ErrorIs(t, err, domain.ErrApplicationNotFound)
	})

	t.Run("interview round-trips every field in UTC", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		iv := interviewFor(p, base.Add(123*time.Microsecond))
		require.NoError(t, interviews.Create(ctx, iv))
		got, err := interviews.Get(ctx, iv.InterviewID)
		require.NoError(t, err)
		assert.Equal(t, iv, got)
		assert.Equal(t, time.UTC, got.ScheduledAt.Location())
	})

	t.Run("unknown interview is not found", func(t *testing.T) {
		_, err := interviews.Get(ctx, uuid.NewString())
		require.ErrorIs(t, err, domain.ErrInterviewNotFound)
	})

	t.Run("several interviews per application, listed soonest first and paged", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		later := interviewFor(p, base.Add(48*time.Hour))
		soonest := interviewFor(p, base)
		middle := interviewFor(p, base.Add(24*time.Hour))
		for _, iv := range []domain.Interview{later, soonest, middle} {
			require.NoError(t, interviews.Create(ctx, iv))
		}
		other := seedParticipants(t, ctx, pool)
		require.NoError(t, interviews.Create(ctx, interviewFor(other, base)), "same time on another application is fine")

		all, total, err := interviews.ListByApplication(ctx, p.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, []string{soonest.InterviewID, middle.InterviewID, later.InterviewID},
			[]string{all[0].InterviewID, all[1].InterviewID, all[2].InterviewID})

		page2, total, err := interviews.ListByApplication(ctx, p.ApplicationID, domain.Page{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, 3, total, "total_count is the full count on every page")
		require.Len(t, page2, 1)
		assert.Equal(t, later.InterviewID, page2[0].InterviewID)
	})

	t.Run("same application at the same moment is ALREADY_EXISTS, in any time zone", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		first := interviewFor(p, base)
		require.NoError(t, interviews.Create(ctx, first))
		sameMoment := interviewFor(p, base.In(time.FixedZone("BTT", 6*3600)))
		require.ErrorIs(t, interviews.Create(ctx, sameMoment), domain.ErrInterviewExists)
		_, total, err := interviews.ListByApplication(ctx, p.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
	})

	t.Run("simultaneous bookings for the same time: exactly one is stored", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		errs := make([]error, 5)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = interviews.Create(ctx, interviewFor(p, base.Add(time.Hour)))
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			assert.ErrorIs(t, err, domain.ErrInterviewExists)
		}
		assert.Equal(t, 1, ok)
	})

	t.Run("database refuses long details and unknown applications", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		long := interviewFor(p, base)
		long.Details = strings.Repeat("a", 1001)
		err := interviews.Create(ctx, long)
		require.Error(t, err)
		assert.Equal(t, "create interview failed", err.Error(), "driver details are hidden")

		ok := interviewFor(p, base)
		ok.Details = strings.Repeat("ཀ", 1000)
		require.NoError(t, interviews.Create(ctx, ok), "1000 multibyte characters fit")

		ghost := interviewFor(domain.ApplicationParticipants{ApplicationID: uuid.NewString()}, base)
		require.Error(t, interviews.Create(ctx, ghost))
	})

	t.Run("application with no interviews gets an empty list", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		got, total, err := interviews.ListByApplication(ctx, p.ApplicationID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
