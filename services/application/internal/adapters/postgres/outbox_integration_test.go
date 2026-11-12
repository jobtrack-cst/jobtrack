//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type publishFunc func(context.Context, *eventsv1.EventEnvelope) error

func (f publishFunc) Publish(c context.Context, e *eventsv1.EventEnvelope) error { return f(c, e) }

type clock struct{}

func (clock) Now() time.Time { return time.Now().UTC() }

// Reuses the running PostgreSQL from the repository integration test.
func checkOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	r := postgres.NewRepository(pool)
	_, err := pool.Exec(ctx, "TRUNCATE applications, outbox_events")
	require.NoError(t, err)
	a := domain.Application{ID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString(), JobID: uuid.NewString(), ResumeID: uuid.NewString(), CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	a.UpdatedAt = a.CreatedAt
	// Hold the writer inside its transaction: the relay and readers see nothing.
	gate, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = gate.Exec(ctx, "SELECT pg_advisory_lock(580058)")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION gate_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(580058); RETURN NEW; END $$; CREATE TRIGGER gate_outbox BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION gate_outbox()`)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() { _, e := r.Create(ctx, a); result <- e }()
	require.Eventually(t, func() bool {
		var waiting int
		err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND objid=580058 AND NOT granted").Scan(&waiting)
		return err == nil && waiting == 1
	}, 5*time.Second, 10*time.Millisecond)
	_, err = r.Get(ctx, a.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	before, err := r.Pending(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, before)
	_, err = gate.Exec(ctx, "SELECT pg_advisory_unlock(580058)")
	require.NoError(t, err)
	gate.Release()
	require.NoError(t, <-result)
	_, err = pool.Exec(ctx, "DROP TRIGGER gate_outbox ON outbox_events; DROP FUNCTION gate_outbox(); TRUNCATE applications,outbox_events")
	require.NoError(t, err)
	// A deferred failure at commit also rolls back both records.
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic commit failure'; END $$; CREATE CONSTRAINT TRIGGER fail_commit AFTER INSERT ON outbox_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_commit()`)
	require.NoError(t, err)
	_, err = r.Create(ctx, a)
	require.Error(t, err)
	_, err = r.Get(ctx, a.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_commit ON outbox_events; DROP FUNCTION fail_commit()")
	require.NoError(t, err)
	// Inject outbox write failure: neither operation may leave a database change.
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic outbox failure'; END $$;
 CREATE TRIGGER fail_outbox BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION fail_outbox()`)
	require.NoError(t, err)
	_, err = r.Create(ctx, a)
	require.Error(t, err)
	_, err = r.Get(ctx, a.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events DISABLE TRIGGER fail_outbox")
	require.NoError(t, err)
	saved, err := r.Create(ctx, a)
	require.NoError(t, err)
	require.Equal(t, a, saved)
	pending, err := r.Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	eventID := pending[0].EventId
	require.Equal(t, a.UpdatedAt, pending[0].OccurredAt.AsTime())
	require.Equal(t, "application", pending[0].Producer)
	var submitted eventsv1.ApplicationSubmitted
	require.NoError(t, pending[0].Data.UnmarshalTo(&submitted))
	require.Equal(t, a.ID, submitted.ApplicationId)
	require.Equal(t, a.ApplicantID, submitted.ApplicantId)
	require.Equal(t, a.JobID, submitted.JobId)
	require.Equal(t, a.ResumeID, submitted.ResumeId)
	require.Equal(t, a.EmployerID, submitted.EmployerId)
	// Producer data has exactly the shared fixture's protobuf shape.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", "application.submitted.json"))
	require.NoError(t, err)
	fixture, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	require.Equal(t, fixture.Data.TypeUrl, pending[0].Data.TypeUrl)
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events ENABLE TRIGGER fail_outbox")
	require.NoError(t, err)
	_, err = r.Withdraw(ctx, a.ID, a.ApplicantID)
	require.Error(t, err)
	saved, err = r.Get(ctx, a.ID)
	require.NoError(t, err)
	require.False(t, saved.Withdrawn)
	pending, err = r.Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_outbox ON outbox_events; DROP FUNCTION fail_outbox()")
	require.NoError(t, err)
	withdrawn, err := r.Withdraw(ctx, a.ID, a.ApplicantID)
	require.NoError(t, err)
	require.True(t, withdrawn.Withdrawn)
	pending, err = r.Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.NotEqual(t, eventID, pending[1].EventId)
	var withdrawal eventsv1.ApplicationWithdrawn
	require.NoError(t, pending[1].Data.UnmarshalTo(&withdrawal))
	require.Equal(t, a.ID, withdrawal.ApplicationId)
	require.Equal(t, a.ApplicantID, withdrawal.ApplicantId)
	require.Equal(t, a.JobID, withdrawal.JobId)
	require.Equal(t, withdrawn.UpdatedAt, pending[1].OccurredAt.AsTime())
	// A duplicate/race writes no extra event.
	_, err = r.Withdraw(ctx, a.ID, a.ApplicantID)
	require.ErrorIs(t, err, domain.ErrWithdrawn)
	_, err = r.Create(ctx, a)
	require.ErrorIs(t, err, domain.ErrDuplicate)
	// Post-commit broker failure cannot undo the saved application or lose events.
	failed := publishFunc(func(_ context.Context, e *eventsv1.EventEnvelope) error {
		require.True(t, proto.Equal(pending[0], e))
		return errors.New("broker down")
	})
	n, err := app.NewOutboxRelay(r, failed, clock{}).PublishPending(ctx)
	require.Error(t, err)
	require.Zero(t, n)
	again, err := r.Pending(ctx, 50)
	require.NoError(t, err)
	require.True(t, proto.Equal(pending[0], again[0]))
	// Mark failure after successful publish retries the exact envelope.
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_mark() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic marking failure'; END $$; CREATE TRIGGER fail_mark BEFORE UPDATE ON outbox_events FOR EACH ROW EXECUTE FUNCTION fail_mark()`)
	require.NoError(t, err)
	var delivered []*eventsv1.EventEnvelope
	success := publishFunc(func(_ context.Context, e *eventsv1.EventEnvelope) error { delivered = append(delivered, e); return nil })
	relay := app.NewOutboxRelay(r, success, clock{})
	n, err = relay.PublishPending(ctx)
	require.Error(t, err)
	require.Zero(t, n)
	_, err = pool.Exec(ctx, "DROP TRIGGER fail_mark ON outbox_events; DROP FUNCTION fail_mark()")
	require.NoError(t, err)
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Len(t, delivered, 3)
	require.True(t, proto.Equal(delivered[0], delivered[1]))
	require.NoError(t, r.MarkPublished(ctx, eventID, time.Now()))
	pending, err = r.Pending(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, pending)
	_, err = r.Pending(ctx, 0)
	require.Error(t, err)
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = r.Pending(canceled, 1)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, r.MarkPublished(canceled, eventID, time.Now()), context.Canceled)
	// Corrupt persisted payloads fail closed rather than marking or skipping events.
	_, err = pool.Exec(ctx, "UPDATE outbox_events SET payload='invalid',published_at=NULL WHERE event_id=$1", eventID)
	require.NoError(t, err)
	_, err = r.Pending(ctx, 1)
	require.EqualError(t, err, "stored outbox event is not valid")
}
