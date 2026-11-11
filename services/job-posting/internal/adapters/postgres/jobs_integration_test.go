//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	employerA = "11111111-1111-4111-8111-111111111111"
	employerB = "22222222-2222-4222-8222-222222222222"
	categoryX = "33333333-3333-4333-8333-333333333333"
	tag1      = "44444444-4444-4444-8444-444444444444"
	tag2      = "55555555-5555-4555-8555-555555555555"
	unknownID = "00000000-0000-4000-8000-000000000000"
)

var base = time.Date(2026, 11, 11, 6, 0, 0, 0, time.UTC)

// startJobDB follows the employer pattern (JT-32): a real Postgres with all
// migrations applied, run twice to prove they are safe on every startup.
func startJobDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("job_posting_test"), pgcontainer.WithUsername("job_posting_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func newJob(employerID string, status domain.Status, created, deadline time.Time) domain.Job {
	return domain.Job{
		EmployerID: employerID, Status: status, CreatedAt: created, UpdatedAt: created,
		JobFields: domain.JobFields{
			Title: "Junior Go Developer", Description: "Build services.", CategoryID: categoryX,
			TagIDs: []string{tag2, tag1}, Location: "Thimphu", Deadline: deadline,
		},
	}
}

func ids(jobs []domain.Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.ID
	}
	return out
}

func TestJobs_Integration(t *testing.T) {
	ctx, pool := startJobDB(t)
	repo := postgres.NewJobs(pool)

	t.Run("create round-trips every field and generates the id", func(t *testing.T) {
		in := newJob(employerA, domain.StatusDraft, base, base.Add(48*time.Hour+123*time.Microsecond))
		created, err := repo.Create(ctx, in)
		require.NoError(t, err)
		_, err = domain.NormalizeJobID(created.ID)
		require.NoError(t, err, "database generates a UUID")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created, got)
		assert.Equal(t, employerA, got.EmployerID)
		assert.Equal(t, categoryX, got.CategoryID)
		assert.Equal(t, []string{tag2, tag1}, got.TagIDs, "tag order is kept")
		assert.Equal(t, in.Deadline, got.Deadline)
		assert.Equal(t, time.UTC, got.Deadline.Location())
		assert.Equal(t, domain.StatusDraft, got.Status)
	})

	t.Run("empty category and tags round-trip as empty", func(t *testing.T) {
		in := newJob(employerA, domain.StatusDraft, base, base.Add(time.Hour))
		in.CategoryID, in.TagIDs = "", nil
		created, err := repo.Create(ctx, in)
		require.NoError(t, err)
		assert.Equal(t, "", created.CategoryID)
		assert.Equal(t, []string{}, created.TagIDs)
	})

	t.Run("get unknown job is not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, unknownID)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
	})

	t.Run("database enforces the field limits", func(t *testing.T) {
		tests := []struct {
			name   string
			edit   func(*domain.Job)
			wantOK bool
		}{
			{"title 150", func(j *domain.Job) { j.Title = strings.Repeat("a", 150) }, true},
			{"title 151", func(j *domain.Job) { j.Title = strings.Repeat("a", 151) }, false},
			{"title 150 multibyte", func(j *domain.Job) { j.Title = strings.Repeat("ཀ", 150) }, true},
			{"title empty", func(j *domain.Job) { j.Title = "" }, false},
			{"description 5000", func(j *domain.Job) { j.Description = strings.Repeat("d", 5000) }, true},
			{"description 5001", func(j *domain.Job) { j.Description = strings.Repeat("d", 5001) }, false},
			{"location 100", func(j *domain.Job) { j.Location = strings.Repeat("l", 100) }, true},
			{"location 101", func(j *domain.Job) { j.Location = strings.Repeat("l", 101) }, false},
			{"20 tags", func(j *domain.Job) { j.TagIDs = repeatTag(20) }, true},
			{"21 tags", func(j *domain.Job) { j.TagIDs = repeatTag(21) }, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				j := newJob(employerB, domain.StatusDraft, base, base.Add(time.Hour))
				tt.edit(&j)
				_, err := repo.Create(ctx, j)
				if tt.wantOK {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
		// Clean up so the list tests below see only their own rows for employer B.
		_, err := pool.Exec(ctx, `DELETE FROM jobs WHERE employer_id = $1`, employerB)
		require.NoError(t, err)
	})

	t.Run("update fields only in the expected status", func(t *testing.T) {
		created, err := repo.Create(ctx, newJob(employerA, domain.StatusDraft, base, base.Add(time.Hour)))
		require.NoError(t, err)

		changed := created
		changed.Title, changed.CategoryID, changed.TagIDs = "Senior Go Developer", "", []string{tag1}
		changed.UpdatedAt = base.Add(time.Minute)
		updated, err := repo.UpdateFields(ctx, changed, domain.StatusDraft)
		require.NoError(t, err)
		assert.Equal(t, "Senior Go Developer", updated.Title)
		assert.Equal(t, "", updated.CategoryID)
		assert.Equal(t, []string{tag1}, updated.TagIDs)
		assert.Equal(t, base, updated.CreatedAt, "created_at never changes")
		assert.Equal(t, base.Add(time.Minute), updated.UpdatedAt)
		assert.Equal(t, domain.StatusDraft, updated.Status)

		_, err = repo.UpdateFields(ctx, changed, domain.StatusPublished)
		require.ErrorIs(t, err, domain.ErrJobStatusChanged)

		changed.ID = unknownID
		_, err = repo.UpdateFields(ctx, changed, domain.StatusDraft)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
	})

	t.Run("update status follows the from status", func(t *testing.T) {
		created, err := repo.Create(ctx, newJob(employerA, domain.StatusDraft, base, base.Add(time.Hour)))
		require.NoError(t, err)

		published, err := repo.UpdateStatus(ctx, created.ID, domain.StatusDraft, domain.StatusPublished, base.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, domain.StatusPublished, published.Status)
		assert.Equal(t, base.Add(time.Minute), published.UpdatedAt)

		_, err = repo.UpdateStatus(ctx, created.ID, domain.StatusDraft, domain.StatusPublished, base)
		require.ErrorIs(t, err, domain.ErrJobStatusChanged, "second publish")

		closed, err := repo.UpdateStatus(ctx, created.ID, domain.StatusPublished, domain.StatusClosed, base.Add(2*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, domain.StatusClosed, closed.Status)

		_, err = repo.UpdateStatus(ctx, unknownID, domain.StatusDraft, domain.StatusPublished, base)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
	})

	t.Run("concurrent publish succeeds exactly once", func(t *testing.T) {
		created, err := repo.Create(ctx, newJob(employerA, domain.StatusDraft, base, base.Add(time.Hour)))
		require.NoError(t, err)
		const n = 8
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = repo.UpdateStatus(ctx, created.ID, domain.StatusDraft, domain.StatusPublished, base)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else {
				require.ErrorIs(t, err, domain.ErrJobStatusChanged)
			}
		}
		assert.Equal(t, 1, ok)
	})

	t.Run("list by employer is newest first with filter and pagination", func(t *testing.T) {
		var want []string
		for i, st := range []domain.Status{domain.StatusDraft, domain.StatusPublished, domain.StatusClosed} {
			j, err := repo.Create(ctx, newJob(employerB, st, base.Add(time.Duration(i)*time.Hour), base.Add(48*time.Hour)))
			require.NoError(t, err)
			want = append([]string{j.ID}, want...)
		}

		all, total, err := repo.ListByEmployer(ctx, employerB, domain.StatusUnspecified, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, want, ids(all), "newest first, employer A rows excluded")

		first, total, err := repo.ListByEmployer(ctx, employerB, domain.StatusUnspecified, domain.Page{Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, want[:2], ids(first))

		second, _, err := repo.ListByEmployer(ctx, employerB, domain.StatusUnspecified, domain.Page{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, want[2:], ids(second))

		beyond, total, err := repo.ListByEmployer(ctx, employerB, domain.StatusUnspecified, domain.Page{Limit: 2, Offset: 10})
		require.NoError(t, err)
		assert.Empty(t, beyond)
		assert.Equal(t, 3, total, "total is filled past the last page")

		closed, total, err := repo.ListByEmployer(ctx, employerB, domain.StatusClosed, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, want[:1], ids(closed))

		none, total, err := repo.ListByEmployer(ctx, unknownID, domain.StatusUnspecified, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Equal(t, []domain.Job{}, none)
	})

	t.Run("list closing between uses a half-open window on published jobs", func(t *testing.T) {
		window := domain.ClosingWindow{Start: base.Add(100 * time.Hour), End: base.Add(148 * time.Hour)}
		create := func(st domain.Status, deadline time.Time) string {
			j, err := repo.Create(ctx, newJob(employerA, st, base, deadline))
			require.NoError(t, err)
			return j.ID
		}
		atStart := create(domain.StatusPublished, window.Start)
		later := create(domain.StatusPublished, window.Start.Add(10*time.Hour))
		lastMicro := create(domain.StatusPublished, window.End.Add(-time.Microsecond))
		create(domain.StatusPublished, window.End)                          // excluded: end is exclusive
		create(domain.StatusPublished, window.Start.Add(-time.Microsecond)) // excluded: before start
		create(domain.StatusDraft, window.Start.Add(time.Hour))             // excluded: not published
		create(domain.StatusClosed, window.Start.Add(time.Hour))            // excluded: not published
		create(domain.StatusRemoved, window.Start.Add(time.Hour))           // excluded: not published

		got, total, err := repo.ListClosingBetween(ctx, window, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, []string{atStart, later, lastMicro}, ids(got), "ordered by deadline")

		page2, total, err := repo.ListClosingBetween(ctx, window, domain.Page{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, []string{lastMicro}, ids(page2))
	})
}

func repeatTag(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = tag1
	}
	return out
}
