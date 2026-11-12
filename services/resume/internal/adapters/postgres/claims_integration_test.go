//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
)

func seedOutgoing(t *testing.T, ctx context.Context, pool *pgxpool.Pool, created time.Time) (string, string) {
	t.Helper()
	f := newFile(applicantA, created, pdf(100))
	event := uploaded(t, f)
	_, err := postgres.NewResumes(pool).Create(ctx, f, event)
	require.NoError(t, err)
	body, err := events.MarshalEnvelope(event)
	require.NoError(t, err)
	return event.EventId, string(body)
}

func TestOutbox_IntegrationLifecycle(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewOutbox(pool)
	firstID, original := seedOutgoing(t, ctx, pool, base)
	secondID, _ := seedOutgoing(t, ctx, pool, base.Add(time.Second))
	// Claim reads a database-relative expiry, not the old event timestamp.
	var before time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&before))
	first, found, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, firstID, first.ID)
	require.Equal(t, original, first.Payload)
	require.Equal(t, "resume.uploaded", first.Type)
	require.NoError(t, domain.ValidateOutboxClaim(first.ID, first.ClaimToken))
	require.WithinDuration(t, before.Add(time.Minute), first.LeaseUntil, time.Second)
	// Returned claim has committed and released its lock and pool connection.
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT event_id FROM outgoing_events WHERE event_id=$1 FOR UPDATE NOWAIT`, firstID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	conn.Release()
	second, found, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondID, second.ID)
	empty, found, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, empty)
	// Expire the durable lease using database time; a new adapter models restart.
	_, err = pool.Exec(ctx, `UPDATE outgoing_events SET lease_until=statement_timestamp()-INTERVAL '1 second' WHERE event_id=$1`, first.ID)
	require.NoError(t, err)
	reclaimed, found, err := postgres.NewOutbox(pool).Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ID, reclaimed.ID)
	require.Equal(t, original, reclaimed.Payload)
	require.NotEqual(t, first.ClaimToken, reclaimed.ClaimToken)
	for _, tc := range []struct {
		name string
		call func() (bool, error)
	}{
		{"stale completion", func() (bool, error) { return repo.MarkPublished(ctx, first.ID, first.ClaimToken) }},
		{"stale retry", func() (bool, error) { return repo.Retry(ctx, first.ID, first.ClaimToken, 0) }},
		{"unknown event", func() (bool, error) { return repo.MarkPublished(ctx, uuid.NewString(), reclaimed.ClaimToken) }},
	} {
		t.Run(tc.name, func(t *testing.T) { changed, err := tc.call(); require.NoError(t, err); require.False(t, changed) })
	}
	var currentToken string
	require.NoError(t, pool.QueryRow(ctx, `SELECT claim_token FROM outgoing_events WHERE event_id=$1`, first.ID).Scan(&currentToken))
	require.Equal(t, reclaimed.ClaimToken, currentToken)
	changed, err := repo.Retry(ctx, first.ID, reclaimed.ClaimToken, time.Hour)
	require.NoError(t, err)
	require.True(t, changed)
	// Retry timing survives construction of another repository instance.
	_, found, err = postgres.NewOutbox(pool).Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.False(t, found)
	var next time.Time
	var token *string
	var lease *time.Time
	var published *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT next_attempt_at, claim_token, lease_until, published_at FROM outgoing_events WHERE event_id=$1`, first.ID).Scan(&next, &token, &lease, &published))
	require.Nil(t, token)
	require.Nil(t, lease)
	require.Nil(t, published)
	var databaseNow time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
	require.WithinDuration(t, databaseNow.Add(time.Hour), next, time.Second)
	_, err = pool.Exec(ctx, `UPDATE outgoing_events SET next_attempt_at=statement_timestamp()-INTERVAL '1 second' WHERE event_id=$1`, first.ID)
	require.NoError(t, err)
	retry, found, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ID, retry.ID)
	require.Equal(t, original, retry.Payload)
	changed, err = repo.Retry(ctx, retry.ID, retry.ClaimToken, 0)
	require.NoError(t, err)
	require.True(t, changed)
	retry, found, err = repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	changed, err = repo.MarkPublished(ctx, retry.ID, retry.ClaimToken)
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = repo.MarkPublished(ctx, retry.ID, retry.ClaimToken)
	require.NoError(t, err)
	require.False(t, changed)
	changed, err = repo.Retry(ctx, retry.ID, retry.ClaimToken, 0)
	require.NoError(t, err)
	require.False(t, changed)
	var stored string
	var retryAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload, published_at, claim_token, lease_until, next_attempt_at FROM outgoing_events WHERE event_id=$1`, retry.ID).Scan(&stored, &published, &token, &lease, &retryAt))
	require.Equal(t, original, stored)
	require.NotNil(t, published)
	require.Nil(t, token)
	require.Nil(t, lease)
	require.Nil(t, retryAt)
	changed, err = repo.MarkPublished(ctx, second.ID, second.ClaimToken)
	require.NoError(t, err)
	require.True(t, changed)
	_, found, err = repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.False(t, found)
}

func TestOutbox_IntegrationConcurrencyAndRollback(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewOutbox(pool)
	const workers = 8
	for i := 0; i < workers; i++ {
		seedOutgoing(t, ctx, pool, base)
	}
	// Lock the earliest candidate in another transaction to prove SKIP LOCKED.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	var lockedID string
	require.NoError(t, tx.QueryRow(ctx, `SELECT event_id FROM outgoing_events ORDER BY created_at,event_id LIMIT 1 FOR UPDATE`).Scan(&lockedID))
	type result struct {
		event domain.OutgoingEvent
		found bool
		err   error
	}
	results := make(chan result, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e, f, err := repo.Claim(ctx, time.Minute)
			results <- result{e, f, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	seen := map[string]bool{}
	for r := range results {
		require.NoError(t, r.err)
		if r.found {
			require.NotEqual(t, lockedID, r.event.ID)
			require.False(t, seen[r.event.ID])
			seen[r.event.ID] = true
		}
	}
	require.Len(t, seen, workers-1)
	require.NoError(t, tx.Rollback(ctx))
	last, found, err := repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, lockedID, last.ID)
	for id := range seen {
		var tok string
		require.NoError(t, pool.QueryRow(ctx, `SELECT claim_token FROM outgoing_events WHERE event_id=$1`, id).Scan(&tok))
		_, err := repo.MarkPublished(ctx, id, tok)
		require.NoError(t, err)
	}
	_, err = repo.MarkPublished(ctx, last.ID, last.ClaimToken)
	require.NoError(t, err)
	freshID, body := seedOutgoing(t, ctx, pool, base)
	// Force claim UPDATE failure in this disposable database. No partial claim commits.
	_, err = pool.Exec(ctx, `ALTER TABLE outgoing_events ADD CONSTRAINT test_refuse_claim CHECK (claim_token IS NULL)`)
	require.NoError(t, err)
	got, found, err := repo.Claim(ctx, time.Minute)
	require.Error(t, err)
	require.False(t, found)
	require.Empty(t, got)
	require.Equal(t, "claim outgoing event failed", err.Error())
	var token *string
	var lease *time.Time
	var stored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT claim_token,lease_until,payload FROM outgoing_events WHERE event_id=$1`, freshID).Scan(&token, &lease, &stored))
	require.Nil(t, token)
	require.Nil(t, lease)
	require.Equal(t, body, stored)
	_, err = pool.Exec(ctx, `ALTER TABLE outgoing_events DROP CONSTRAINT test_refuse_claim`)
	require.NoError(t, err)
	got, found, err = repo.Claim(ctx, time.Minute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, freshID, got.ID)
	// Constraint enforces pairing even for direct writes.
	_, err = pool.Exec(ctx, `UPDATE outgoing_events SET lease_until=NULL WHERE event_id=$1`, freshID)
	require.Error(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = repo.Claim(canceled, time.Minute)
	require.ErrorIs(t, err, context.Canceled)
	// An expired but unreplaced token may still complete safely.
	_, err = pool.Exec(ctx, `UPDATE outgoing_events SET lease_until=statement_timestamp()-INTERVAL '1 second' WHERE event_id=$1`, freshID)
	require.NoError(t, err)
	changed, err := repo.MarkPublished(ctx, got.ID, got.ClaimToken)
	require.NoError(t, err)
	require.True(t, changed)
}
