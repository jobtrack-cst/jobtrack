//go:build integration

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func startConsumerDB(t *testing.T) (context.Context, *app.Consumer, *postgres.SearchRepository) {
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

	writeRepo := postgres.NewWriteRepository(pool)
	searchRepo := postgres.NewSearchRepository(pool)
	return ctx, app.NewConsumer(writeRepo), searchRepo
}

func loadFixture(t *testing.T, name string) *eventsv1.EventEnvelope {
	t.Helper()
	// Need to go up 5 directories from the postgres adapter test to reach repo root
	path := filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	env, err := events.UnmarshalEnvelope(data)
	require.NoError(t, err)
	return env
}

func TestConsumer_Events(t *testing.T) {
	ctx, consumer, searchRepo := startConsumerDB(t)

	// 1. Load the published fixture
	pubEnv := loadFixture(t, "job.published.json")
	var pubData eventsv1.JobPublished
	require.NoError(t, pubEnv.Data.UnmarshalTo(&pubData))

	// 2. Process published event
	err := consumer.ProcessEvent(ctx, pubEnv.EventId, pubEnv.Type, pubEnv.OccurredAt.AsTime(), &pubData)
	require.NoError(t, err)

	// 3. Verify it's in the search model
	jobs, _, err := searchRepo.Search(ctx, domain.Filters{PageSize: 10})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, pubData.JobId, jobs[0].JobID)
	assert.Equal(t, pubData.Title, jobs[0].Title)

	// 4. Try to process the SAME event again (idempotency)
	err = consumer.ProcessEvent(ctx, pubEnv.EventId, pubEnv.Type, pubEnv.OccurredAt.AsTime(), &pubData)
	require.NoError(t, err)

	// 5. Send an "update" event with an OLDER timestamp (should be ignored)
	updatedData := eventsv1.JobUpdated{
		JobId: pubData.JobId, EmployerId: pubData.EmployerId,
		Title: "OLDER TITLE", Location: "Old Location", Deadline: pubData.Deadline,
	}
	oldTime := pubEnv.OccurredAt.AsTime().Add(-time.Hour)
	err = consumer.ProcessEvent(ctx, "11111111-1111-1111-1111-111111111111", "job.updated", oldTime, &updatedData)
	require.NoError(t, err)

	// Verify the title did NOT change
	jobs, _, err = searchRepo.Search(ctx, domain.Filters{PageSize: 10})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, pubData.Title, jobs[0].Title)

	// 6. Send an "update" event with a NEWER timestamp (should update)
	updatedData.Title = "NEWER TITLE"
	newTime := pubEnv.OccurredAt.AsTime().Add(time.Hour)
	err = consumer.ProcessEvent(ctx, "22222222-2222-2222-2222-222222222222", "job.updated", newTime, &updatedData)
	require.NoError(t, err)

	// Verify the title DID change
	jobs, _, err = searchRepo.Search(ctx, domain.Filters{PageSize: 10})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "NEWER TITLE", jobs[0].Title)

	// 7. Send a "closed" event
	closedEnv := loadFixture(t, "job.closed.json")
	var closedData eventsv1.JobClosed
	require.NoError(t, closedEnv.Data.UnmarshalTo(&closedData))

	// Ensure the job ID matches our test job
	closedData.JobId = pubData.JobId

	err = consumer.ProcessEvent(ctx, closedEnv.EventId, closedEnv.Type, closedEnv.OccurredAt.AsTime(), &closedData)
	require.NoError(t, err)

	// 8. Verify the job is no longer in search (because it's closed)
	jobs, _, err = searchRepo.Search(ctx, domain.Filters{PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, jobs)
}
