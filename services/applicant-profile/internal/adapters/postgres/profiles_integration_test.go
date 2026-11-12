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
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var base = time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)

// startProfileDB follows the job-posting pattern (JT-38): a real Postgres with
// all migrations applied, run twice to prove they are safe on every startup.
func startProfileDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("applicant_profile_test"), pgcontainer.WithUsername("applicant_profile_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func content(tag string) domain.ProfileContent {
	return domain.ProfileContent{
		Summary: "Summary " + tag + "\nwith a second line",
		Education: []domain.Education{
			{Institution: "RUB " + tag, Qualification: "BSc", StartYear: 2019, EndYear: 2022},
			{Institution: "CST " + tag, Qualification: "BE", StartYear: 2023, EndYear: 0},
		},
		Skills: []string{"Go " + tag, "SQL " + tag, "React " + tag},
	}
}

func newProfile(tag string) domain.Profile {
	return domain.Profile{ApplicantID: uuid.NewString(), ProfileContent: content(tag), CreatedAt: base, UpdatedAt: base}
}

func TestProfiles_Integration(t *testing.T) {
	ctx, pool := startProfileDB(t)
	repo := postgres.NewProfiles(pool)

	t.Run("create round-trips every field in order", func(t *testing.T) {
		p := newProfile("a")
		p.CreatedAt, p.UpdatedAt = base.Add(123*time.Microsecond), base.Add(123*time.Microsecond)
		require.NoError(t, repo.Create(ctx, p))
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
		assert.Equal(t, time.UTC, got.CreatedAt.Location())
		assert.Equal(t, int32(0), got.Education[1].EndYear, "ongoing study keeps end_year 0")
	})

	t.Run("empty profile reads back with empty, non-nil lists", func(t *testing.T) {
		p := domain.Profile{ApplicantID: uuid.NewString(), ProfileContent: domain.ProfileContent{Education: []domain.Education{}, Skills: []string{}}, CreatedAt: base, UpdatedAt: base}
		require.NoError(t, repo.Create(ctx, p))
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
		assert.NotNil(t, got.Education)
		assert.NotNil(t, got.Skills)
	})

	t.Run("second create is ALREADY_EXISTS and changes nothing", func(t *testing.T) {
		p := newProfile("b")
		require.NoError(t, repo.Create(ctx, p))
		again := p
		again.ProfileContent = content("other")
		require.ErrorIs(t, repo.Create(ctx, again), domain.ErrProfileExists)
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
	})

	t.Run("unknown applicant is not found", func(t *testing.T) {
		_, err := repo.Get(ctx, uuid.NewString())
		require.ErrorIs(t, err, domain.ErrProfileNotFound)
		_, err = repo.Replace(ctx, uuid.NewString(), content("x"), base)
		require.ErrorIs(t, err, domain.ErrProfileNotFound)
	})

	t.Run("replace swaps every field and keeps created_at", func(t *testing.T) {
		p := newProfile("c")
		require.NoError(t, repo.Create(ctx, p))
		next := domain.ProfileContent{
			Summary:   "New summary",
			Education: []domain.Education{{Institution: "Only one", Qualification: "MSc", StartYear: 2027, EndYear: 2029}},
			Skills:    []string{"Rust"},
		}
		later := base.Add(time.Hour)
		replaced, err := repo.Replace(ctx, p.ApplicantID, next, later)
		require.NoError(t, err)
		want := domain.Profile{ApplicantID: p.ApplicantID, ProfileContent: next, CreatedAt: base, UpdatedAt: later}
		assert.Equal(t, want, replaced)
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, want, got, "old education and skills are gone")
	})

	t.Run("replace with empty content clears every field", func(t *testing.T) {
		p := newProfile("d")
		require.NoError(t, repo.Create(ctx, p))
		empty := domain.ProfileContent{Education: []domain.Education{}, Skills: []string{}}
		_, err := repo.Replace(ctx, p.ApplicantID, empty, base.Add(time.Minute))
		require.NoError(t, err)
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		assert.Equal(t, empty, got.ProfileContent)
	})

	t.Run("a skill only differing in case may be re-added on replace", func(t *testing.T) {
		p := newProfile("e")
		require.NoError(t, repo.Create(ctx, p))
		next := content("e")
		next.Skills = []string{strings.ToUpper(p.Skills[0])}
		_, err := repo.Replace(ctx, p.ApplicantID, next, base.Add(time.Minute))
		require.NoError(t, err)
	})

	t.Run("database limits roll back the whole write", func(t *testing.T) {
		tests := []struct {
			name string
			edit func(*domain.ProfileContent)
		}{
			{"summary 2001", func(c *domain.ProfileContent) { c.Summary = strings.Repeat("a", 2001) }},
			{"institution 151", func(c *domain.ProfileContent) { c.Education[0].Institution = strings.Repeat("a", 151) }},
			{"end before start", func(c *domain.ProfileContent) { c.Education[0].EndYear = 2018 }},
			{"start before 1950", func(c *domain.ProfileContent) { c.Education[0].StartYear = 1949 }},
			{"skill 51", func(c *domain.ProfileContent) { c.Skills[0] = strings.Repeat("a", 51) }},
			{"duplicate skill ignoring case", func(c *domain.ProfileContent) { c.Skills[1] = strings.ToUpper(c.Skills[0]) }},
			{"21 education entries", func(c *domain.ProfileContent) {
				for len(c.Education) < 21 {
					c.Education = append(c.Education, c.Education[0])
				}
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := newProfile("f")
				require.NoError(t, repo.Create(ctx, p))
				bad := content("f")
				tt.edit(&bad)
				_, err := repo.Replace(ctx, p.ApplicantID, bad, base.Add(time.Minute))
				require.Error(t, err)
				assert.Equal(t, "update applicant profile failed", err.Error(), "driver details are hidden")
				got, err := repo.Get(ctx, p.ApplicantID)
				require.NoError(t, err)
				assert.Equal(t, p, got, "nothing changed")

				fresh := newProfile("g")
				fresh.ProfileContent = bad
				require.Error(t, repo.Create(ctx, fresh))
				_, err = repo.Get(ctx, fresh.ApplicantID)
				require.ErrorIs(t, err, domain.ErrProfileNotFound, "failed create leaves no profile row")
			})
		}
	})

	t.Run("concurrent replaces never mix their lists", func(t *testing.T) {
		p := newProfile("h")
		require.NoError(t, repo.Create(ctx, p))
		tags := []string{"w1", "w2", "w3", "w4"}
		var wg sync.WaitGroup
		errs := make([]error, len(tags))
		for i, tag := range tags {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = repo.Replace(ctx, p.ApplicantID, content(tag), base.Add(time.Duration(i+1)*time.Minute))
			}()
		}
		wg.Wait()
		for _, err := range errs {
			require.NoError(t, err)
		}
		got, err := repo.Get(ctx, p.ApplicantID)
		require.NoError(t, err)
		matched := false
		for _, tag := range tags {
			if assert.ObjectsAreEqual(content(tag), got.ProfileContent) {
				matched = true
			}
		}
		assert.True(t, matched, "final profile is exactly one writer's content: %+v", got.ProfileContent)
	})
}
