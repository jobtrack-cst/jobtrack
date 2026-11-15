//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func startPostgres(t *testing.T) (context.Context, *postgres.OutboxRepository, *postgres.Jobs) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine",
		pgcontainer.WithDatabase("job_posting_test"),
		pgcontainer.WithUsername("job_posting_test"),
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
	url, err := container.ConnectionString(ctx, "sslmode=disable&timezone=UTC")
	require.NoError(t, err)
	pool, err := db.OpenPool(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")))

	return ctx, postgres.NewOutboxRepository(pool), postgres.NewJobs(pool)
}

func TestOutbox_SaveAndPending(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)

	employerID := "11111111-1111-4111-8111-111111111111"
	j := domain.Job{
		EmployerID: employerID,
		JobFields: domain.JobFields{
			Title: "Test Job", Description: "Desc", CategoryID: "", TagIDs: []string{}, Location: "Thimphu", Deadline: time.Now().Add(24 * time.Hour),
		},
		Status:    domain.StatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	created, err := jobs.Create(ctx, j)
	require.NoError(t, err)

	published, err := jobs.UpdateStatus(ctx, created.ID, domain.StatusDraft, domain.StatusPublished, time.Now())
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPublished, published.Status)

	pending, err := outbox.Pending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "job.published", pending[0].Type)
	assert.Equal(t, "job-posting", pending[0].Producer)

	err = outbox.MarkPublished(ctx, pending[0].EventId, time.Now())
	require.NoError(t, err)

	pendingAfter, err := outbox.Pending(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, pendingAfter)
}

func TestOutbox_RecordJobRemoved(t *testing.T) {
	ctx, _, jobs := startPostgres(t)

	employerID := "11111111-1111-4111-8111-111111111111"
	j := domain.Job{
		EmployerID: employerID,
		JobFields: domain.JobFields{
			Title: "Test Job", Description: "Desc", CategoryID: "", TagIDs: []string{}, Location: "Thimphu", Deadline: time.Now().Add(24 * time.Hour),
		},
		Status:    domain.StatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	created, err := jobs.Create(ctx, j)
	require.NoError(t, err)

	eventID := "22222222-2222-4222-8222-222222222222"
	err = jobs.RecordJobRemoved(ctx, eventID, created.ID)
	require.NoError(t, err)

	updated, err := jobs.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRemoved, updated.Status)

	err = jobs.RecordJobRemoved(ctx, eventID, created.ID)
	require.NoError(t, err)
}
