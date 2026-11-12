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

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

func countOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&n))
	return n
}

// createAndApprove stores a pending request and returns it approved at "at".
func createAndApprove(t *testing.T, ctx context.Context, repo *postgres.Requests, n int, employer string, at time.Time) domain.VerificationRequest {
	t.Helper()
	v := newPending(n, employer, base)
	_, err := repo.Create(ctx, v)
	require.NoError(t, err)
	require.NoError(t, v.Approve(adminID, at))
	return v
}

func TestOutbox_DecisionStoresEventForTheRelay(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo, outbox := postgres.NewRequests(pool), postgres.NewOutbox(pool)

	v := createAndApprove(t, ctx, repo, 1, employerID, base.Add(time.Hour))
	event := eventFor(t, v)
	_, err := repo.Decide(ctx, v, event)
	require.NoError(t, err)

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

// No event may exist for a decision that was not committed.
func TestOutbox_FailedDecisionStoresNoEvent(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)

	approved := createAndApprove(t, ctx, repo, 1, employerID, base.Add(time.Hour))
	_, err := repo.Decide(ctx, approved, eventFor(t, approved))
	require.NoError(t, err)
	require.Equal(t, 1, countOutbox(t, ctx, pool))

	t.Run("request already decided", func(t *testing.T) {
		again := newPending(1, employerID, base)
		require.NoError(t, again.Reject(adminID, "late", base.Add(2*time.Hour)))
		_, err := repo.Decide(ctx, again, eventFor(t, again))
		require.ErrorIs(t, err, domain.ErrRequestNotPending)
		require.Equal(t, 1, countOutbox(t, ctx, pool))
	})
	t.Run("unknown request", func(t *testing.T) {
		missing := newPending(9, employerID, base)
		missing.ID = unknownID
		require.NoError(t, missing.Approve(adminID, base.Add(2*time.Hour)))
		_, err := repo.Decide(ctx, missing, eventFor(t, missing))
		require.ErrorIs(t, err, domain.ErrRequestNotFound)
		require.Equal(t, 1, countOutbox(t, ctx, pool))
	})
	t.Run("second approval rolls back with the unique index", func(t *testing.T) {
		second := createAndApprove(t, ctx, repo, 2, employerID, base.Add(2*time.Hour))
		_, err := repo.Decide(ctx, second, eventFor(t, second))
		require.ErrorIs(t, err, domain.ErrAlreadyVerified)
		require.Equal(t, 1, countOutbox(t, ctx, pool))
		got, err := repo.GetByID(ctx, second.ID)
		require.NoError(t, err)
		require.Equal(t, domain.StatusPending, got.Status)
	})
	t.Run("invalid event leaves the request pending", func(t *testing.T) {
		v := createAndApprove(t, ctx, repo, 3, otherID, base.Add(2*time.Hour))
		_, err := repo.Decide(ctx, v, nil)
		require.Error(t, err)
		got, err := repo.GetByID(ctx, v.ID)
		require.NoError(t, err)
		require.Equal(t, domain.StatusPending, got.Status)
		require.Equal(t, 1, countOutbox(t, ctx, pool))
	})
	t.Run("duplicate event ID rolls back the decision", func(t *testing.T) {
		v := createAndApprove(t, ctx, repo, 4, "d0000000-0000-4000-8000-000000000001", base.Add(2*time.Hour))
		e := eventFor(t, v)
		var existing string
		require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text FROM outbox_events LIMIT 1`).Scan(&existing))
		e.EventId = existing
		_, err := repo.Decide(ctx, v, e)
		require.Error(t, err)
		require.False(t, errors.Is(err, domain.ErrAlreadyVerified))
		got, err := repo.GetByID(ctx, v.ID)
		require.NoError(t, err)
		require.Equal(t, domain.StatusPending, got.Status)
	})
}

func TestOutbox_PendingIsOldestFirstAndLimited(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo, outbox := postgres.NewRequests(pool), postgres.NewOutbox(pool)
	employers := []string{
		"e0000000-0000-4000-8000-000000000001",
		"e0000000-0000-4000-8000-000000000002",
		"e0000000-0000-4000-8000-000000000003",
	}
	// Decided at +3h, +1h, +2h, so the oldest first order is 2, 3, 1.
	var ids []string
	for i, at := range []time.Duration{3 * time.Hour, time.Hour, 2 * time.Hour} {
		v := createAndApprove(t, ctx, repo, i+1, employers[i], base.Add(at))
		e := eventFor(t, v)
		_, err := repo.Decide(ctx, v, e)
		require.NoError(t, err)
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
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)
	r := newPending(1, employerID, base)
	_, err := repo.Create(ctx, r)
	require.NoError(t, err)
	approve, reject := r, r
	require.NoError(t, approve.Approve(adminID, base.Add(time.Minute)))
	require.NoError(t, reject.Reject(adminID, "duplicate", base.Add(time.Minute)))
	approveEvent, rejectEvent := eventFor(t, approve), eventFor(t, reject)

	done := make(chan error, 2)
	go func() { _, err := repo.Decide(ctx, approve, approveEvent); done <- err }()
	go func() { _, err := repo.Decide(ctx, reject, rejectEvent); done <- err }()
	wins := 0
	for range 2 {
		if err := <-done; err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, domain.ErrRequestNotPending)
		}
	}
	require.Equal(t, 1, wins)
	require.Equal(t, 1, countOutbox(t, ctx, pool), "only the winning decision has an event")
}
