//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	empA       = "11111111-1111-4111-8111-111111111111"
	catX       = "33333333-3333-4333-8333-333333333333"
	tag1       = "44444444-4444-4444-8444-444444444444"
	tag2       = "55555555-5555-4555-8555-555555555555"
	jobOpen1   = "a0000000-0000-4000-8000-000000000001"
	jobOpen2   = "a0000000-0000-4000-8000-000000000002"
	jobClosed  = "a0000000-0000-4000-8000-000000000003"
	jobExpired = "a0000000-0000-4000-8000-000000000004"
)

func startSearchDB(t *testing.T) (context.Context, *postgres.SearchRepository) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine",
		pgcontainer.WithDatabase("job_search_test"),
		pgcontainer.WithUsername("job_search_test"),
		pgcontainer.WithPassword("synthetic-test-password"),
		pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			_ = container.Terminate(cleanupCtx)
		})
	}
	require.NoError(t, err)
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := db.OpenPool(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")))

	// Seed: two open jobs (one matches all filters), one closed, one expired.
	_, err = pool.Exec(ctx, `
INSERT INTO jobs (job_id, employer_id, title, category_id, tag_ids, location, deadline, closed) VALUES
($1,$2,'Backend Engineer',$3,ARRAY[$4,$5],'Thimphu', now() + interval '1 day', FALSE),
($6,$2,'Frontend Engineer',$3,ARRAY[$5],'Phuntsholing', now() + interval '2 day', FALSE),
($7,$2,'Old Backend Role',$3,ARRAY[$4],'Thimphu', now() + interval '1 day', TRUE),
($8,$2,'Expired Backend Role',$3,ARRAY[$4],'Thimphu', now() - interval '1 day', FALSE)`,
		jobOpen1, empA, catX, tag1, tag2,
		jobOpen2, jobClosed, jobExpired)
	require.NoError(t, err)

	return ctx, postgres.NewSearchRepository(pool)
}

func TestSearch_OnlyOpenSortedByDeadline(t *testing.T) {
	ctx, repo := startSearchDB(t)
	jobs, total, err := repo.Search(ctx, domain.Filters{PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, jobs, 2)
	assert.Equal(t, jobOpen1, jobs[0].JobID) // deadline 1 day < 2 day
	assert.Equal(t, jobOpen2, jobs[1].JobID)
}

func TestSearch_QueryCaseInsensitive(t *testing.T) {
	ctx, repo := startSearchDB(t)
	jobs, total, err := repo.Search(ctx, domain.Filters{Query: "backend", PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, jobs, 1)
	assert.Equal(t, jobOpen1, jobs[0].JobID)
}

func TestSearch_CategoryAndLocationAndTags(t *testing.T) {
	ctx, repo := startSearchDB(t)
	tests := []struct {
		name string
		f    domain.Filters
		want []string
	}{
		{"category", domain.Filters{CategoryID: catX, PageSize: 50}, []string{jobOpen1, jobOpen2}},
		{"location", domain.Filters{Location: "Thimphu", PageSize: 50}, []string{jobOpen1}},
		{"tag OR", domain.Filters{TagIDs: []string{tag2}, PageSize: 50}, []string{jobOpen1, jobOpen2}},
		{"tags+query AND", domain.Filters{Query: "engineer", TagIDs: []string{tag1}, PageSize: 50}, []string{jobOpen1}},
		{"no match", domain.Filters{Query: "nurse", PageSize: 50}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, _, err := repo.Search(ctx, tt.f)
			require.NoError(t, err)
			got := make([]string, len(jobs))
			for i, j := range jobs {
				got[i] = j.JobID
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSearch_Pagination(t *testing.T) {
	ctx, repo := startSearchDB(t)
	page, total, err := repo.Search(ctx, domain.Filters{PageSize: 1, PageOffset: 1})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, page, 1)
	assert.Equal(t, jobOpen2, page[0].JobID)
}
