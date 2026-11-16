//go:build integration

package postgres_test

import (
	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestApplicationConsumer_IdempotentAndRollback(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	handler := rabbitmq.ApplicationHandler(postgres.NewParticipants(pool))
	require.NoError(t, handler(ctx, e))
	require.NoError(t, handler(ctx, e))
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events`).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM application_participants`).Scan(&n))
	require.Equal(t, 1, n)
	id := uuid.NewString()
	require.Error(t, postgres.NewParticipants(pool).RecordApplication(ctx, id, domain.ApplicationParticipants{ApplicationID: "11111111-1111-4111-8111-111111111111", ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id=$1`, id).Scan(&n))
	require.Zero(t, n)
}
func TestMessages_OutboxAtomicity(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	p := domain.ApplicationParticipants{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString()}
	require.NoError(t, postgres.NewParticipants(pool).RecordApplication(ctx, uuid.NewString(), p))
	m := domain.Message{ID: uuid.NewString(), ApplicationID: p.ApplicationID, SenderID: p.ApplicantID, RecipientID: p.EmployerID, Content: "hello", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	m.UpdatedAt = m.CreatedAt
	_, err := pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_event CHECK(false)`)
	require.NoError(t, err)
	_, err = postgres.NewMessages(pool).Create(ctx, m)
	require.Error(t, err)
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&n))
	require.Zero(t, n)
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events DROP CONSTRAINT reject_event`)
	require.NoError(t, err)
	_, err = postgres.NewMessages(pool).Create(ctx, m)
	require.NoError(t, err)
	pending, err := postgres.NewOutbox(pool).Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.NotEqual(t, m.ID, pending[0].EventId)
	var data eventsv1.MessageSent
	require.NoError(t, pending[0].Data.UnmarshalTo(&data))
	require.Equal(t, m.ID, data.MessageId)
	require.Equal(t, m.RecipientID, data.RecipientId)
	require.NoError(t, postgres.NewOutbox(pool).MarkPublished(ctx, pending[0].EventId, time.Now()))
	pending, err = postgres.NewOutbox(pool).Pending(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, pending)
}
