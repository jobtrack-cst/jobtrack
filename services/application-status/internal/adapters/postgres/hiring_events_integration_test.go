//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

func hiringEvent(appID string, kind domain.HiringKind, at time.Time) domain.HiringEvent {
	e := domain.HiringEvent{EventID: uuid.NewString(), ApplicationID: appID, Kind: kind, OccurredAt: at}
	if kind >= domain.HiringOfferMade {
		e.ApplicantID = applicantID
	}
	return e
}

// history returns the application's new_status values in order.
func history(t *testing.T, ctx context.Context, pool *pgxpool.Pool, appID string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT new_status FROM status_history WHERE application_id = $1 ORDER BY occurred_at, history_id`, appID)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

// published returns "PREVIOUS>NEW" for every status_changed of the application, in outbox order.
func published(t *testing.T, ctx context.Context, pool *pgxpool.Pool, appID string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT (payload->'data'->>'previous_status') || '>' || (payload->'data'->>'new_status')
		FROM outbox_events WHERE payload->'data'->>'application_id' = $1 ORDER BY outbox_id`, appID)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

func claimed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventID string) bool {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id = $1`, eventID).Scan(&n))
	return n == 1
}

func currentStatus(t *testing.T, ctx context.Context, repo *postgres.Statuses, appID string) domain.Status {
	t.Helper()
	s, err := repo.Get(ctx, appID)
	require.NoError(t, err)
	return s.Status
}

func TestHiringEvents_Integration(t *testing.T) {
	ctx, pool := startStatusDB(t)
	repo := postgres.NewStatuses(pool)

	t.Run("full path to ACCEPTED, with UNDER_REVIEW and a second interview", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		at := base.Add(time.Hour)
		for _, kind := range []domain.HiringKind{domain.HiringReviewShortlisted, domain.HiringInterviewScheduled,
			domain.HiringInterviewScheduled, domain.HiringOfferMade, domain.HiringOfferAccepted} {
			at = at.Add(time.Hour)
			require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, kind, at)))
		}
		assert.Equal(t, []string{"submitted", "under_review", "shortlisted", "interview", "offered", "accepted"},
			history(t, ctx, pool, s.ApplicationID))
		assert.Equal(t, []string{"UNSPECIFIED>SUBMITTED", "SUBMITTED>UNDER_REVIEW", "UNDER_REVIEW>SHORTLISTED",
			"SHORTLISTED>INTERVIEW", "INTERVIEW>OFFERED", "OFFERED>ACCEPTED"}, published(t, ctx, pool, s.ApplicationID))
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusAccepted, got.Status)
		assert.Equal(t, base.Add(6*time.Hour), got.UpdatedAt)
	})

	t.Run("rejected from SUBMITTED in one step; declined from OFFERED", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringReviewRejected, base.Add(time.Hour))))
		assert.Equal(t, []string{"UNSPECIFIED>SUBMITTED", "SUBMITTED>REJECTED"}, published(t, ctx, pool, s.ApplicationID))

		o := start(t, ctx, repo, base)
		for _, kind := range []domain.HiringKind{domain.HiringReviewShortlisted, domain.HiringInterviewScheduled, domain.HiringOfferMade, domain.HiringOfferDeclined} {
			require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(o.ApplicationID, kind, base.Add(time.Hour))))
		}
		assert.Equal(t, domain.StatusDeclined, currentStatus(t, ctx, repo, o.ApplicationID))
	})

	t.Run("a repeated event_id changes nothing", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		e := hiringEvent(s.ApplicationID, domain.HiringReviewShortlisted, base.Add(time.Hour))
		for range 3 {
			require.NoError(t, repo.ApplyHiringEvent(ctx, e))
		}
		assert.Len(t, history(t, ctx, pool, s.ApplicationID), 3)
		assert.Len(t, published(t, ctx, pool, s.ApplicationID), 3)
	})

	t.Run("a further interview claims the event_id and writes nothing else", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringReviewShortlisted, base.Add(time.Hour))))
		require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringInterviewScheduled, base.Add(2*time.Hour))))
		second := hiringEvent(s.ApplicationID, domain.HiringInterviewScheduled, base.Add(3*time.Hour))
		require.NoError(t, repo.ApplyHiringEvent(ctx, second))
		assert.True(t, claimed(t, ctx, pool, second.EventID))
		assert.Len(t, history(t, ctx, pool, s.ApplicationID), 4)
		got, err := repo.Get(ctx, s.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, base.Add(2*time.Hour), got.UpdatedAt, "updated_at is unchanged")
	})

	t.Run("a skipped step is refused, keeps no claim and can apply later", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		interview := hiringEvent(s.ApplicationID, domain.HiringInterviewScheduled, base.Add(time.Hour))
		require.ErrorIs(t, repo.ApplyHiringEvent(ctx, interview), domain.ErrInvalidTransition)
		assert.False(t, claimed(t, ctx, pool, interview.EventID))
		assert.Equal(t, []string{"submitted"}, history(t, ctx, pool, s.ApplicationID))
		require.ErrorIs(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringOfferMade, base.Add(time.Hour))), domain.ErrInvalidTransition)
		require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringReviewShortlisted, base.Add(2*time.Hour))))
		require.NoError(t, repo.ApplyHiringEvent(ctx, interview), "the same event_id applies once the step is valid")
		assert.Equal(t, domain.StatusInterview, currentStatus(t, ctx, repo, s.ApplicationID))
	})

	t.Run("nothing leaves a final state", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, domain.HiringReviewRejected, base.Add(time.Hour))))
		for _, kind := range []domain.HiringKind{domain.HiringReviewShortlisted, domain.HiringReviewRejected, domain.HiringInterviewScheduled, domain.HiringOfferMade} {
			require.ErrorIs(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, kind, base.Add(2*time.Hour))), domain.ErrInvalidTransition)
		}
		assert.Equal(t, domain.StatusRejected, currentStatus(t, ctx, repo, s.ApplicationID))
	})

	t.Run("an offer for another applicant is a conflict", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		for _, kind := range []domain.HiringKind{domain.HiringReviewShortlisted, domain.HiringInterviewScheduled} {
			require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, kind, base.Add(time.Hour))))
		}
		e := hiringEvent(s.ApplicationID, domain.HiringOfferMade, base.Add(2*time.Hour))
		e.ApplicantID = uuid.NewString()
		require.ErrorIs(t, repo.ApplyHiringEvent(ctx, e), domain.ErrConflictingEvent)
		assert.False(t, claimed(t, ctx, pool, e.EventID))
		assert.Equal(t, domain.StatusInterview, currentStatus(t, ctx, repo, s.ApplicationID))
	})

	t.Run("an unknown application is not found and keeps no claim", func(t *testing.T) {
		e := hiringEvent(uuid.NewString(), domain.HiringReviewShortlisted, base)
		require.ErrorIs(t, repo.ApplyHiringEvent(ctx, e), domain.ErrApplicationNotFound)
		assert.False(t, claimed(t, ctx, pool, e.EventID))
	})

	t.Run("simultaneous deliveries of one event apply once", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		e := hiringEvent(s.ApplicationID, domain.HiringReviewShortlisted, base.Add(time.Hour))
		var wg sync.WaitGroup
		errs := make([]error, 8)
		for i := range errs {
			wg.Add(1)
			go func() { defer wg.Done(); errs[i] = repo.ApplyHiringEvent(ctx, e) }()
		}
		wg.Wait()
		for _, err := range errs {
			require.NoError(t, err)
		}
		assert.Len(t, history(t, ctx, pool, s.ApplicationID), 3)
	})

	t.Run("accepted and declined at once: exactly one applies", func(t *testing.T) {
		s := start(t, ctx, repo, base)
		for _, kind := range []domain.HiringKind{domain.HiringReviewShortlisted, domain.HiringInterviewScheduled, domain.HiringOfferMade} {
			require.NoError(t, repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, kind, base.Add(time.Hour))))
		}
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, kind := range []domain.HiringKind{domain.HiringOfferAccepted, domain.HiringOfferDeclined} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = repo.ApplyHiringEvent(ctx, hiringEvent(s.ApplicationID, kind, base.Add(2*time.Hour)))
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else {
				require.ErrorIs(t, err, domain.ErrInvalidTransition)
			}
		}
		assert.Equal(t, 1, ok)
		assert.True(t, currentStatus(t, ctx, repo, s.ApplicationID).IsFinal())
		assert.Len(t, history(t, ctx, pool, s.ApplicationID), 6)
	})

	t.Run("canceled context hides driver details", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		err := repo.ApplyHiringEvent(canceled, hiringEvent(uuid.NewString(), domain.HiringReviewShortlisted, base))
		require.ErrorIs(t, err, context.Canceled)
	})
}

// A failing outbox insert rolls back status, history and the claim, so both
// steps of SHORTLISTED are stored together or not at all.
func TestHiringEvents_Atomicity_Integration(t *testing.T) {
	ctx, pool := startStatusDB(t)
	repo := postgres.NewStatuses(pool)
	s := start(t, ctx, repo, base)
	_, err := pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_hiring CHECK (payload->'data'->>'new_status' <> 'SHORTLISTED') NOT VALID`)
	require.NoError(t, err)
	e := hiringEvent(s.ApplicationID, domain.HiringReviewShortlisted, base.Add(time.Hour))
	require.EqualError(t, repo.ApplyHiringEvent(ctx, e), "apply hiring event failed", "driver details are hidden")
	assert.False(t, claimed(t, ctx, pool, e.EventID))
	assert.Equal(t, []string{"submitted"}, history(t, ctx, pool, s.ApplicationID))
	assert.Equal(t, domain.StatusSubmitted, currentStatus(t, ctx, repo, s.ApplicationID))
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events DROP CONSTRAINT reject_hiring`)
	require.NoError(t, err)
	require.NoError(t, repo.ApplyHiringEvent(ctx, e))
	assert.Equal(t, domain.StatusShortlisted, currentStatus(t, ctx, repo, s.ApplicationID))
}
