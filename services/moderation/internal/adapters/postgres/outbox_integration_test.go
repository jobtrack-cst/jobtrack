//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

func countOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&n))
	return n
}

func TestOutbox_RemovalStoresEventForTheRelay(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo, outbox := postgres.NewReports(pool), postgres.NewOutbox(pool)

	rm := removal(jobA, base.Add(time.Hour))
	event := eventFor(t, rm)
	require.NoError(t, repo.RemoveJob(ctx, rm, event))

	var routingKey string
	var createdAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT routing_key, created_at FROM outbox_events WHERE event_id = $1`, event.GetEventId()).Scan(&routingKey, &createdAt))
	require.Equal(t, "moderation.job_removed", routingKey)
	require.True(t, createdAt.Equal(rm.RemovedAt), "created_at is the removal time")

	pending, err := outbox.Pending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.True(t, proto.Equal(event, pending[0]), "the relay reads back exactly the stored envelope")

	require.NoError(t, outbox.MarkPublished(ctx, event.GetEventId(), base.Add(2*time.Hour)))
	pending, err = outbox.Pending(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, pending, "a published event is not read again")

	require.NoError(t, outbox.MarkPublished(ctx, event.GetEventId(), base.Add(3*time.Hour)), "marking twice is not an error")
	var publishedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT published_at FROM outbox_events WHERE event_id = $1`, event.GetEventId()).Scan(&publishedAt))
	require.True(t, publishedAt.Equal(base.Add(2*time.Hour)), "the first published time is kept")
}

// No event may exist for a removal that was not committed.
func TestOutbox_FailedRemovalStoresNoEvent(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	require.NoError(t, removeJob(t, ctx, repo, removal(jobA, base)))
	require.Equal(t, 1, countOutbox(t, ctx, pool))

	t.Run("second takedown of the same job", func(t *testing.T) {
		err := removeJob(t, ctx, repo, removal(jobA, base.Add(time.Hour)))
		require.ErrorIs(t, err, domain.ErrJobAlreadyRemoved)
		require.Equal(t, 1, countOutbox(t, ctx, pool))
	})
	t.Run("invalid event leaves the job and its reports untouched", func(t *testing.T) {
		open := mustCreate(t, ctx, repo, newOpen(1, jobB, reporterID, base))
		require.Error(t, repo.RemoveJob(ctx, removal(jobB, base), nil))
		require.Equal(t, domain.StatusOpen, mustGet(t, ctx, repo, open.ID).Status)
		// The job was not removed, so it can still be reported and removed.
		require.NoError(t, removeJob(t, ctx, repo, removal(jobB, base.Add(time.Hour))))
		require.Equal(t, 2, countOutbox(t, ctx, pool))
	})
	t.Run("duplicate event ID rolls back the removal", func(t *testing.T) {
		job := "f0000000-0000-4000-8000-000000000001"
		rm := removal(job, base)
		e := eventFor(t, rm)
		var existing string
		require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text FROM outbox_events LIMIT 1`).Scan(&existing))
		e.EventId = existing
		err := repo.RemoveJob(ctx, rm, e)
		require.Error(t, err)
		require.False(t, errors.Is(err, domain.ErrJobAlreadyRemoved))
		var removed bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM removed_jobs WHERE job_id = $1)`, job).Scan(&removed))
		require.False(t, removed, "the takedown must roll back with its event")
	})
	t.Run("event with another routing key is refused by the CHECK", func(t *testing.T) {
		job := "f0000000-0000-4000-8000-000000000002"
		rm := removal(job, base)
		e := eventFor(t, rm)
		e.Type = "job.closed"
		require.Error(t, repo.RemoveJob(ctx, rm, e))
		require.Equal(t, 2, countOutbox(t, ctx, pool))
	})
}

func TestOutbox_PendingIsOldestFirstAndLimited(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo, outbox := postgres.NewReports(pool), postgres.NewOutbox(pool)
	jobs := []string{
		"e1000000-0000-4000-8000-000000000001",
		"e1000000-0000-4000-8000-000000000002",
		"e1000000-0000-4000-8000-000000000003",
	}
	// Removed at +3h, +1h, +2h, so the oldest first order is 2, 3, 1.
	var ids []string
	for i, at := range []time.Duration{3 * time.Hour, time.Hour, 2 * time.Hour} {
		rm := removal(jobs[i], base.Add(at))
		e := eventFor(t, rm)
		require.NoError(t, repo.RemoveJob(ctx, rm, e))
		ids = append(ids, e.GetEventId())
	}

	pending, err := outbox.Pending(ctx, 2)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.Equal(t, []string{ids[1], ids[2]}, []string{pending[0].GetEventId(), pending[1].GetEventId()})

	_, err = outbox.Pending(ctx, 0)
	require.Error(t, err)
}

func TestOutbox_ConcurrentAdminsStoreOneEvent(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	first, second := removal(jobA, base), removal(jobA, base.Add(time.Minute))
	firstEvent, secondEvent := eventFor(t, first), eventFor(t, second)

	done := make(chan error, 2)
	go func() { done <- repo.RemoveJob(ctx, first, firstEvent) }()
	go func() { done <- repo.RemoveJob(ctx, second, secondEvent) }()
	wins := 0
	for range 2 {
		if err := <-done; err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, domain.ErrJobAlreadyRemoved)
		}
	}
	require.Equal(t, 1, wins)
	require.Equal(t, 1, countOutbox(t, ctx, pool), "only the winning takedown has an event")
}
