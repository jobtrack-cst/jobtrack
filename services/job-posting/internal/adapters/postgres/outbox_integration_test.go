//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const testEmployerID = "11111111-1111-4111-8111-111111111111"

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

// createJob stores a job directly in the given status, so no outbox row is
// written for it and only RecordJobRemoved can add one.
func createJob(ctx context.Context, t *testing.T, jobs *postgres.Jobs, status domain.Status) domain.Job {
	t.Helper()
	now := time.Now().UTC()
	created, err := jobs.Create(ctx, domain.Job{
		EmployerID: testEmployerID,
		JobFields: domain.JobFields{
			Title: "Test Job", Description: "Desc", CategoryID: "", TagIDs: []string{},
			Location: "Thimphu", Deadline: now.Add(24 * time.Hour),
		},
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)
	return created
}

// closedEvents returns the unpublished job.closed outbox events for one job.
func closedEvents(ctx context.Context, t *testing.T, outbox *postgres.OutboxRepository, jobID string) []*eventsv1.EventEnvelope {
	t.Helper()
	pending, err := outbox.Pending(ctx, 1000)
	require.NoError(t, err)
	var out []*eventsv1.EventEnvelope
	for _, e := range pending {
		if e.Type != "job.closed" {
			continue
		}
		var data eventsv1.JobClosed
		require.NoError(t, e.Data.UnmarshalTo(&data))
		if data.JobId == jobID {
			out = append(out, e)
		}
	}
	return out
}

func TestOutbox_SaveAndPending(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)

	j := domain.Job{
		EmployerID: testEmployerID,
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

func TestJobs_RecordJobRemoved(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)

	t.Run("published job is removed and stores exactly one job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)

		require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID))

		removed, err := jobs.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRemoved, removed.Status)
		closed := closedEvents(ctx, t, outbox, job.ID)
		require.Len(t, closed, 1)
		assert.Equal(t, "job-posting", closed[0].Producer)
		_, err = uuid.Parse(closed[0].EventId)
		assert.NoError(t, err)
		assert.True(t, removed.UpdatedAt.Equal(closed[0].OccurredAt.AsTime()),
			"occurred_at %v should equal the removal time %v", closed[0].OccurredAt.AsTime(), removed.UpdatedAt)
	})

	t.Run("redelivered event_id stores no second job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)
		eventID := uuid.NewString()

		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, job.ID))
		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, job.ID))

		assert.Len(t, closedEvents(ctx, t, outbox, job.ID), 1)
	})

	t.Run("already removed job changes nothing and stores no job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)
		require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID))
		before, err := jobs.GetByID(ctx, job.ID)
		require.NoError(t, err)

		require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID))

		after, err := jobs.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRemoved, after.Status)
		assert.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "updated_at must not change")
		assert.Len(t, closedEvents(ctx, t, outbox, job.ID), 1)
	})

	t.Run("unknown job stores nothing", func(t *testing.T) {
		jobID := uuid.NewString()
		eventID := uuid.NewString()

		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, jobID))
		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, jobID))

		_, err := jobs.GetByID(ctx, jobID)
		assert.ErrorIs(t, err, domain.ErrJobNotFound)
		assert.Empty(t, closedEvents(ctx, t, outbox, jobID))
	})

	t.Run("draft job is removed without job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusDraft)

		require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID))

		removed, err := jobs.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRemoved, removed.Status)
		assert.Empty(t, closedEvents(ctx, t, outbox, job.ID))
	})

	t.Run("closed job is removed without a second job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusClosed)

		require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID))

		removed, err := jobs.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRemoved, removed.Status)
		assert.Empty(t, closedEvents(ctx, t, outbox, job.ID))
	})

	t.Run("concurrent removals with different event IDs store one job.closed", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)
		var wg sync.WaitGroup
		errs := make([]error, 5)
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID)
			}(i)
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}
		assert.Len(t, closedEvents(ctx, t, outbox, job.ID), 1)
	})
}

const otherEmployerID = "22222222-2222-4222-8222-222222222222"

// updatedEvents counts the unpublished job.updated outbox events for one job.
func updatedEvents(ctx context.Context, t *testing.T, outbox *postgres.OutboxRepository, jobID string) int {
	t.Helper()
	pending, err := outbox.Pending(ctx, 1000)
	require.NoError(t, err)
	count := 0
	for _, e := range pending {
		if e.Type != "job.updated" {
			continue
		}
		var data eventsv1.JobUpdated
		require.NoError(t, e.Data.UnmarshalTo(&data))
		if data.JobId == jobID {
			count++
		}
	}
	return count
}

func totalPending(ctx context.Context, t *testing.T, outbox *postgres.OutboxRepository) int {
	t.Helper()
	pending, err := outbox.Pending(ctx, 1000)
	require.NoError(t, err)
	return len(pending)
}

// JT-116: a draft edit stores no event; a published edit stores job.updated.
func TestJobs_UpdateFields_EventOnlyForPublished(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)

	t.Run("draft edit stores no event", func(t *testing.T) {
		draft := createJob(ctx, t, jobs, domain.StatusDraft)
		before := totalPending(ctx, t, outbox)
		draft.Title = "Edited draft"
		draft.UpdatedAt = time.Now().UTC()
		updated, err := jobs.UpdateFields(ctx, draft, domain.StatusDraft)
		require.NoError(t, err)
		assert.Equal(t, "Edited draft", updated.Title)
		assert.Equal(t, domain.StatusDraft, updated.Status)
		assert.Equal(t, before, totalPending(ctx, t, outbox))
		assert.Zero(t, updatedEvents(ctx, t, outbox, draft.ID))
	})

	t.Run("published edit stores job.updated", func(t *testing.T) {
		published := createJob(ctx, t, jobs, domain.StatusPublished)
		published.Title = "Edited published"
		published.UpdatedAt = time.Now().UTC()
		_, err := jobs.UpdateFields(ctx, published, domain.StatusPublished)
		require.NoError(t, err)
		assert.Equal(t, 1, updatedEvents(ctx, t, outbox, published.ID))
	})

	t.Run("draft edit after a concurrent publish stores nothing", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)
		before := totalPending(ctx, t, outbox)
		_, err := jobs.UpdateFields(ctx, job, domain.StatusDraft)
		require.ErrorIs(t, err, domain.ErrJobStatusChanged)
		assert.Equal(t, before, totalPending(ctx, t, outbox))
	})
}

// JT-116: Delete removes only a draft of the given employer, in one statement,
// and never stores an event.
func TestJobs_Delete(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)

	t.Run("deletes the employer's draft", func(t *testing.T) {
		draft := createJob(ctx, t, jobs, domain.StatusDraft)
		before := totalPending(ctx, t, outbox)
		require.NoError(t, jobs.Delete(ctx, draft.ID, testEmployerID))

		_, err := jobs.GetByID(ctx, draft.ID)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
		listed, total, err := jobs.ListByEmployer(ctx, testEmployerID, domain.StatusUnspecified, domain.Page{Limit: 100})
		require.NoError(t, err)
		for _, j := range listed {
			assert.NotEqual(t, draft.ID, j.ID)
		}
		assert.Equal(t, len(listed), total)
		assert.Equal(t, before, totalPending(ctx, t, outbox))
	})

	t.Run("deleting twice is not found", func(t *testing.T) {
		draft := createJob(ctx, t, jobs, domain.StatusDraft)
		require.NoError(t, jobs.Delete(ctx, draft.ID, testEmployerID))
		require.ErrorIs(t, jobs.Delete(ctx, draft.ID, testEmployerID), domain.ErrJobNotFound)
	})

	t.Run("unknown job is not found", func(t *testing.T) {
		require.ErrorIs(t, jobs.Delete(ctx, uuid.NewString(), testEmployerID), domain.ErrJobNotFound)
	})

	t.Run("refuses another employer's draft", func(t *testing.T) {
		draft := createJob(ctx, t, jobs, domain.StatusDraft)
		require.ErrorIs(t, jobs.Delete(ctx, draft.ID, otherEmployerID), domain.ErrNotJobOwner)
		kept, err := jobs.GetByID(ctx, draft.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusDraft, kept.Status)
	})

	for _, status := range []domain.Status{domain.StatusPublished, domain.StatusClosed, domain.StatusRemoved} {
		t.Run("refuses a "+status.String()+" job", func(t *testing.T) {
			job := createJob(ctx, t, jobs, status)
			before := totalPending(ctx, t, outbox)
			require.ErrorIs(t, jobs.Delete(ctx, job.ID, testEmployerID), domain.ErrJobNotDeletable)
			kept, err := jobs.GetByID(ctx, job.ID)
			require.NoError(t, err)
			assert.Equal(t, status, kept.Status)
			assert.Equal(t, before, totalPending(ctx, t, outbox))
		})
	}

	t.Run("another employer's published job is not owned", func(t *testing.T) {
		job := createJob(ctx, t, jobs, domain.StatusPublished)
		require.ErrorIs(t, jobs.Delete(ctx, job.ID, otherEmployerID), domain.ErrNotJobOwner)
	})

	// JT-90 rule: an unknown job keeps the claim and changes nothing.
	t.Run("moderation removal after delete is a no-op", func(t *testing.T) {
		draft := createJob(ctx, t, jobs, domain.StatusDraft)
		require.NoError(t, jobs.Delete(ctx, draft.ID, testEmployerID))
		before := totalPending(ctx, t, outbox)
		eventID := uuid.NewString()
		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, draft.ID))
		require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, draft.ID))
		_, err := jobs.GetByID(ctx, draft.ID)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
		assert.Empty(t, closedEvents(ctx, t, outbox, draft.ID))
		assert.Equal(t, before, totalPending(ctx, t, outbox))
	})
}
