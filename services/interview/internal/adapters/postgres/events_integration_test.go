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

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
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
		var data eventsv1.InterviewScheduled
		require.NoError(t, e.GetData().UnmarshalTo(&data))
		if data.GetApplicationId() == applicationID {
			out = append(out, e)
		}
	}
	require.NoError(t, rows.Err())
	return out
}

func TestParticipants_RecordApplication_Integration(t *testing.T) {
	ctx, pool := startInterviewDB(t)
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

func TestInterviews_Outbox_Integration(t *testing.T) {
	ctx, pool := startInterviewDB(t)
	interviews := postgres.NewInterviews(pool)
	outbox := postgres.NewOutbox(pool)

	t.Run("each scheduled interview stores exactly one interview.scheduled", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		first := interviewFor(p, base.Add(time.Hour+7*time.Microsecond))
		second := interviewFor(p, base.Add(48*time.Hour))
		require.NoError(t, interviews.Create(ctx, first))
		require.NoError(t, interviews.Create(ctx, second))
		stored := outboxPayloads(t, ctx, pool, p.ApplicationID)
		require.Len(t, stored, 2)
		for i, iv := range []domain.Interview{first, second} {
			e := stored[i]
			_, err := uuid.Parse(e.GetEventId())
			require.NoError(t, err)
			assert.Equal(t, "interview.scheduled", e.GetType())
			assert.Equal(t, "interview", e.GetProducer())
			assert.Equal(t, iv.CreatedAt, e.GetOccurredAt().AsTime())
			var data eventsv1.InterviewScheduled
			require.NoError(t, e.GetData().UnmarshalTo(&data))
			assert.Equal(t, iv.InterviewID, data.GetInterviewId())
			assert.Equal(t, iv.ScheduledAt, data.GetScheduledAt().AsTime())
		}
	})

	t.Run("same application and time adds no event", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		at := base.Add(3 * time.Hour)
		first := interviewFor(p, at)
		require.NoError(t, interviews.Create(ctx, first))
		require.ErrorIs(t, interviews.Create(ctx, interviewFor(p, at)), domain.ErrInterviewExists)
		stored := outboxPayloads(t, ctx, pool, p.ApplicationID)
		require.Len(t, stored, 1)
		var data eventsv1.InterviewScheduled
		require.NoError(t, stored[0].GetData().UnmarshalTo(&data))
		assert.Equal(t, first.InterviewID, data.GetInterviewId(), "the first interview's event is kept")
	})

	t.Run("simultaneous bookings of one time store exactly one event", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		at := base.Add(5 * time.Hour)
		var wg sync.WaitGroup
		for range 5 {
			wg.Add(1)
			go func() { defer wg.Done(); _ = interviews.Create(ctx, interviewFor(p, at)) }()
		}
		wg.Wait()
		assert.Len(t, outboxPayloads(t, ctx, pool, p.ApplicationID), 1)
	})

	t.Run("relay publishes in order, marks, and publishes nothing twice", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE outbox_events SET published_at = now() WHERE published_at IS NULL`)
		require.NoError(t, err)
		var ids []string
		for i := range 3 {
			p := seedParticipants(t, ctx, pool)
			require.NoError(t, interviews.Create(ctx, interviewFor(p, base.Add(time.Duration(i+1)*time.Hour))))
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
			require.NoError(t, interviews.Create(ctx, interviewFor(p, base.Add(time.Hour))))
		}
		got, err := outbox.Pending(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		_, err = outbox.Pending(ctx, 0)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO outbox_events (event_id, routing_key, payload, created_at) VALUES ($1, 'interview.scheduled', 'not json', now())`, uuid.NewString())
		require.NoError(t, err)
		_, err = outbox.Pending(ctx, 100)
		require.EqualError(t, err, "stored outbox event is not valid")
	})

	t.Run("an interview without times is refused before any write", func(t *testing.T) {
		p := seedParticipants(t, ctx, pool)
		iv := interviewFor(p, time.Time{})
		require.Error(t, interviews.Create(ctx, iv))
		assert.Zero(t, count(t, ctx, pool, `SELECT count(*) FROM interviews WHERE application_id = $1`, p.ApplicationID))
	})
}

// A failing outbox insert rolls the interview back, so an interview never
// exists without its event.
func TestInterviews_OutboxAtomicity_Integration(t *testing.T) {
	ctx, pool := startInterviewDB(t)
	interviews := postgres.NewInterviews(pool)
	p := seedParticipants(t, ctx, pool)
	_, err := pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_event CHECK (false) NOT VALID`)
	require.NoError(t, err)
	iv := interviewFor(p, base.Add(time.Hour))
	err = interviews.Create(ctx, iv)
	require.EqualError(t, err, "create interview failed", "driver details are hidden")
	_, err = interviews.Get(ctx, iv.InterviewID)
	require.ErrorIs(t, err, domain.ErrInterviewNotFound)
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events DROP CONSTRAINT reject_event`)
	require.NoError(t, err)
	require.NoError(t, interviews.Create(ctx, iv))
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
