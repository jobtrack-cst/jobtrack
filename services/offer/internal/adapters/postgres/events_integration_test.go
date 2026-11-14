//go:build integration

package postgres_test

import (
	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
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
func TestOffers_OutboxAtomicity(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	repo := postgres.NewOffers(pool)
	o := syntheticOffer()
	_, err := pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_event CHECK(false)`)
	require.NoError(t, err)
	_, err = repo.Create(ctx, o)
	require.Error(t, err)
	_, err = repo.GetByID(ctx, o.ID)
	require.ErrorIs(t, err, domain.ErrOfferNotFound)
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events DROP CONSTRAINT reject_event`)
	require.NoError(t, err)
	_, err = repo.Create(ctx, o)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE outbox_events ADD CONSTRAINT reject_event CHECK(false) NOT VALID`)
	require.NoError(t, err)
	_, err = repo.Decide(ctx, o.ID, o.ApplicantID, domain.OfferStatusAccepted, o.UpdatedAt)
	require.Error(t, err)
	got, err := repo.GetByID(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, domain.OfferStatusPending, got.Status)
}
