//go:build integration

package postgres_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

func count(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&n))
	return n
}

func outboxPayloads(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationID string) []*eventsv1.EventEnvelope {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT payload FROM outbox_events ORDER BY sequence`)
	require.NoError(t, err)
	defer rows.Close()
	var out []*eventsv1.EventEnvelope
	for rows.Next() {
		var payload string
		require.NoError(t, rows.Scan(&payload))
		e, err := events.UnmarshalEnvelope([]byte(payload))
		require.NoError(t, err)
		var data eventsv1.ReviewCompleted
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		if data.GetApplicationId() == applicationID {
			out = append(out, e)
		}
	}
	require.NoError(t, rows.Err())
	return out
}

func TestParticipants_RecordApplication_Integration(t *testing.T) {
	ctx, pool := startReviewDB(t)
	participants := postgres.NewParticipants(pool)

	t.Run("shared fixture is stored once, however often it is delivered", func(t *testing.T) {
		raw, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
		require.NoError(t, err)
		e, err := events.UnmarshalEnvelope(raw)
		require.NoError(t, err)
		var data eventsv1.ApplicationSubmitted
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		p := domain.ApplicationParticipants{ApplicationID: data.GetApplicationId(), ApplicantID: data.GetApplicantId(), EmployerID: data.GetEmployerId()}
		for range 3 {
			require.NoError(t, participants.RecordApplication(ctx, e.GetEventId(), p))
		}
		assert.Equal(t, 1, count(t, ctx, pool, `SELECT count(*) FROM processed_events WHERE event_id = $1`, e.GetEventId()))
		got, err := participants.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, p, got)
	})

	t.Run("same participants under a new event_id are accepted unchanged", func(t *testing.T) {
		p := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}
		require.NoError(t, participants.RecordApplication(ctx, uuid.NewString(), p))
		require.NoError(t, participants.RecordApplication(ctx, uuid.NewString(), p))
		assert.Equal(t, 1, count(t, ctx, pool, `SELECT count(*) FROM application_participants WHERE application_id = $1`, p.ApplicationID))
	})

	t.Run("conflicting participants are refused and keep no claim", func(t *testing.T) {
		p := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}
		require.NoError(t, participants.RecordApplication(ctx, uuid.NewString(), p))
		for _, changed := range []domain.ApplicationParticipants{
			{ApplicationID: p.ApplicationID, ApplicantID: uuid.NewString(), EmployerID: p.EmployerID},
			{ApplicationID: p.ApplicationID, ApplicantID: p.ApplicantID, EmployerID: uuid.NewString()},
		} {
			eventID := uuid.NewString()
			require.ErrorIs(t, participants.RecordApplication(ctx, eventID, changed), domain.ErrConflictingParticipants)
			assert.Zero(t, count(t, ctx, pool, `SELECT count(*) FROM processed_events WHERE event_id = $1`, eventID))
		}
		got, err := participants.GetByApplicationID(ctx, p.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, p, got, "stored participants are unchanged")
	})

	t.Run("simultaneous deliveries of one event store one claim", func(t *testing.T) {
		p := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}
		eventID := uuid.NewString()
		errs := make([]error, 5)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() { defer wg.Done(); errs[i] = participants.RecordApplication(ctx, eventID, p) }()
		}
		wg.Wait()
		for _, err := range errs {
			require.NoError(t, err)
		}
		assert.Equal(t, 1, count(t, ctx, pool, `SELECT count(*) FROM processed_events WHERE event_id = $1`, eventID))
	})

	t.Run("canceled context hides driver details", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		err := participants.RecordApplication(canceled, uuid.NewString(), domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestReviews_Outbox_Integration(t *testing.T) {
	ctx, pool := startReviewDB(t)
	reviews := postgres.NewReviews(pool)
	outbox := postgres.NewOutbox(pool)

	t.Run("completing a review stores exactly one review.completed", func(t *testing.T) {
		for _, d := range []domain.Decision{domain.DecisionShortlisted, domain.DecisionRejected} {
			p := seedParticipants(t, ctx, pool)
			rv := reviewFor(p, 3, d)
			rv.CreatedAt, rv.UpdatedAt = base.Add(7*time.Microsecond), base.Add(7*time.Microsecond)
			require.NoError(t, reviews.Create(ctx, rv))
			stored := outboxPayloads(t, ctx, pool, p.ApplicationID)
			require.Len(t, stored, 1)
			e := stored[0]
			_, err := uuid.Parse(e.GetEventId())
			require.NoError(t, err)
			assert.Equal(t, "review.completed", e.GetType())
			assert.Equal(t, "review", e.GetProducer())
			assert.Equal(t, rv.CreatedAt, e.GetOccurredAt().AsTime())
			var data eventsv1.ReviewCompleted
			require.NoError(t, e.GetData().UnmarshalTo(&data))
			assert.Equal(t, d.String(), data.GetDecision())
		}
	})

	t.Run("a second review adds no event", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		require.NoError(t, reviews.Create(ctx, reviewFor(p, 2, domain.DecisionRejected)))
		require.ErrorIs(t, reviews.Create(ctx, reviewFor(p, 5, domain.DecisionShortlisted)), domain.ErrReviewExists)
		stored := outboxPayloads(t, ctx, pool, p.ApplicationID)
		require.Len(t, stored, 1)
		var data eventsv1.ReviewCompleted
		require.NoError(t, stored[0].GetData().UnmarshalTo(&data))
		assert.Equal(t, "REJECTED", data.GetDecision(), "the first review's event is kept")
	})

	t.Run("simultaneous reviews store exactly one event", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		var wg sync.WaitGroup
		for i := range 5 {
			wg.Add(1)
			go func() { defer wg.Done(); _ = reviews.Create(ctx, reviewFor(p, int32(i)+1, domain.DecisionShortlisted)) }()
		}
		wg.Wait()
		assert.Len(t, outboxPayloads(t, ctx, pool, p.ApplicationID), 1)
	})

	t.Run("relay publishes in order, marks, and publishes nothing twice", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE outbox_events SET published_at = now() WHERE published_at IS NULL`)
		require.NoError(t, err)
		var ids []string
		for range 3 {
			p := seedParticipants(t, ctx, pool)
			require.NoError(t, reviews.Create(ctx, reviewFor(p, 4, domain.DecisionShortlisted)))
			ids = append(ids, outboxPayloads(t, ctx, pool, p.ApplicationID)[0].GetEventId())
		}
		publisher := &recordingPublisher{}
		relay := app.NewOutboxRelay(outbox, publisher, utcClock{})
		n, err := relay.PublishPending(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, ids, publisher.ids)
		n, err = relay.PublishPending(ctx)
		require.NoError(t, err)
		assert.Zero(t, n)
		assert.Zero(t, count(t, ctx, pool, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`))
		require.NoError(t, outbox.MarkPublished(ctx, ids[0], time.Now()), "marking twice is not an error")
	})

	t.Run("pending honours the limit and refuses a bad limit or payload", func(t *testing.T) {
		for range 2 {
			p := seedParticipants(t, ctx, pool)
			require.NoError(t, reviews.Create(ctx, reviewFor(p, 1, domain.DecisionRejected)))
		}
		got, err := outbox.Pending(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		_, err = outbox.Pending(ctx, 0)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO outbox_events (event_id, routing_key, payload, created_at) VALUES ($1, 'review.completed', 'not json', now())`, uuid.NewString())
		require.NoError(t, err)
		_, err = outbox.Pending(ctx, 100)
		require.EqualError(t, err, "stored outbox event is not valid")
	})
}

// A failing outbox insert rolls the review back, so a review never exists
// without its event.
func TestReviews_OutboxAtomicity_Integration(t *testing.T) {
	ctx, pool := startReviewDB(t)
	reviews := postgres.NewReviews(pool)
	p := seedParticipants(t, ctx, pool)
	_, err := pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_event CHECK (false) NOT VALID`)
	require.NoError(t, err)
	err = reviews.Create(ctx, reviewFor(p, 3, domain.DecisionShortlisted))
	require.EqualError(t, err, "create review failed", "driver details are hidden")
	_, err = reviews.GetByApplicationID(ctx, p.ApplicationID)
	require.ErrorIs(t, err, domain.ErrReviewNotFound)
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events DROP CONSTRAINT reject_event`)
	require.NoError(t, err)
	require.NoError(t, reviews.Create(ctx, reviewFor(p, 3, domain.DecisionShortlisted)))
	assert.Len(t, outboxPayloads(t, ctx, pool, p.ApplicationID), 1)
}

type recordingPublisher struct {
	mu  sync.Mutex
	ids []string
}

func (p *recordingPublisher) Publish(_ context.Context, e *eventsv1.EventEnvelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids = append(p.ids, e.GetEventId())
	return nil
}

type utcClock struct{}

func (utcClock) Now() time.Time { return time.Now().UTC() }
