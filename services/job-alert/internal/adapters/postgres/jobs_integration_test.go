//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/rabbitmq"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestJobs_MatchingCombinationsDuplicatesAndRollback(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	category, tag, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	job := &eventsv1.JobPublished{JobId: uuid.NewString(), CategoryId: category, TagIds: []string{tag}, Location: "Thimphu"}
	for _, tc := range []struct {
		name             string
		categories, tags []string
		location         string
		want             bool
	}{
		{"category", []string{category}, nil, "", true},
		{"tag", nil, []string{other, tag}, "", true},
		{"location", nil, nil, "Thimphu", true},
		{"category tag", []string{category}, []string{tag}, "", true},
		{"category location", []string{category}, nil, "Thimphu", true},
		{"tag location", nil, []string{tag}, "Thimphu", true},
		{"all", []string{category}, []string{tag}, "Thimphu", true},
		{"category miss", []string{other}, []string{tag}, "Thimphu", false},
		{"tag miss", []string{category}, []string{other}, "Thimphu", false},
		{"location miss", []string{category}, []string{tag}, "thimphu", false},
	} {
		a := syntheticAlert(uuid.NewString(), tc.location)
		if tc.categories != nil {
			a.CategoryIDs = tc.categories
		}
		if tc.tags != nil {
			a.TagIDs = tc.tags
		}
		_, err := repo.Create(ctx, a)
		require.NoError(t, err)
	}
	id := uuid.NewString()
	require.NoError(t, repo.RecordJob(ctx, id, job))
	require.NoError(t, repo.RecordJob(ctx, id, job))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM matched_subscriptions").Scan(&count))
	require.Equal(t, 7, count)
	pending, err := repo.Outbox().Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 7)
	// A failure after claiming and inserting a match must roll back all effects.
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events ADD CONSTRAINT fail_test CHECK (false) NOT VALID")
	require.NoError(t, err)
	retryID := uuid.NewString()
	require.Error(t, repo.RecordJob(ctx, retryID, job))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id=$1", retryID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM matched_subscriptions WHERE source_event_id=$1", retryID).Scan(&count))
	require.Zero(t, count)
	_, err = pool.Exec(ctx, "ALTER TABLE outbox_events DROP CONSTRAINT fail_test")
	require.NoError(t, err)
	require.NoError(t, repo.RecordJob(ctx, retryID, job))
	job.Location = "Paro"
	job.CategoryId = other
	job.TagIds = []string{other}
	// Remove subscriptions to exercise a delivery with no matches.
	_, err = pool.Exec(ctx, "DELETE FROM job_alerts")
	require.NoError(t, err)
	emptyID := uuid.NewString()
	require.NoError(t, repo.RecordJob(ctx, emptyID, job))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id=$1", emptyID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM matched_subscriptions WHERE source_event_id=$1", emptyID).Scan(&count))
	require.Zero(t, count)
}

type integrationClock struct{}

func (integrationClock) Now() time.Time { return time.Now().UTC() }

type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, *eventsv1.EventEnvelope) error {
	return errors.New("synthetic broker failure")
}
func TestJobs_OutboxConfirmedDeliveryAndRetry(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	_, err := repo.Create(ctx, syntheticAlert(uuid.NewString(), "Thimphu"))
	require.NoError(t, err)
	require.NoError(t, repo.RecordJob(ctx, uuid.NewString(), &eventsv1.JobPublished{JobId: uuid.NewString(), CategoryId: uuid.NewString(), TagIds: []string{}, Location: "Thimphu"}))
	pending, err := repo.Outbox().Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	id := pending[0].EventId
	relay := app.NewOutboxRelay(repo.Outbox(), failingPublisher{}, integrationClock{})
	n, err := relay.PublishPending(ctx)
	require.Error(t, err)
	require.Zero(t, n)
	broker := startAlertsBroker(t, ctx)
	publisher, err := events.NewPublisher(broker)
	require.NoError(t, err)
	defer publisher.Close()
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "job_alert.matched", "jobtrack.events", false, nil))
	relay = app.NewOutboxRelay(repo.Outbox(), publisher, integrationClock{})
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	delivery, ok, err := ch.Get(q.Name, true)
	require.NoError(t, err)
	require.True(t, ok)
	e, err := events.UnmarshalEnvelope(delivery.Body)
	require.NoError(t, err)
	require.Equal(t, id, e.EventId)
	require.Equal(t, "job-alert", e.Producer)
	require.Equal(t, "job_alert.matched", e.Type)
	var data eventsv1.JobAlertMatched
	require.NoError(t, e.Data.UnmarshalTo(&data))
	require.NotEmpty(t, data.JobId)
	require.NotEmpty(t, data.ApplicantId)
	pending, err = repo.Outbox().Pending(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestJobs_CategorylessEventsMatchOnlyUnrestrictedCategories(t *testing.T) {
	ctx, pool := startAlertsDB(t)
	repo := postgres.NewAlerts(pool)
	raw, err := os.ReadFile("../../../../../tests/contract/fixtures/job.published.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	var job eventsv1.JobPublished
	require.NoError(t, e.Data.UnmarshalTo(&job))
	category, tag := job.CategoryId, job.TagIds[0]
	job.CategoryId = ""
	expectedAlerts, expectedApplicants := []string{}, []string{}
	for _, tc := range []struct {
		name             string
		categories, tags []string
		location         string
		want             bool
	}{
		{"tag only", nil, []string{tag}, "", true},
		{"location only", nil, nil, job.Location, true},
		{"tag and location", nil, []string{tag}, job.Location, true},
		{"category only", []string{category}, nil, "", false},
		{"category tag and location", []string{category}, []string{tag}, job.Location, false},
		{"tag mismatch", nil, []string{uuid.NewString()}, "", false},
		{"location mismatch", nil, nil, "Paro", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := syntheticAlert(uuid.NewString(), tc.location)
			if tc.categories != nil {
				a.CategoryIDs = tc.categories
			}
			if tc.tags != nil {
				a.TagIDs = tc.tags
			}
			_, err := repo.Create(ctx, a)
			require.NoError(t, err)
			if tc.want {
				expectedAlerts = append(expectedAlerts, a.ID)
				expectedApplicants = append(expectedApplicants, a.ApplicantID)
			}
		})
	}
	e.Data, err = anypb.New(&job)
	require.NoError(t, err)
	handler := rabbitmq.JobHandler(repo)
	require.NoError(t, handler(ctx, e))
	require.NoError(t, handler(ctx, e))
	rows, err := pool.Query(ctx, "SELECT alert_id::text FROM matched_subscriptions WHERE source_event_id=$1", e.EventId)
	require.NoError(t, err)
	actualAlerts := []string{}
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		actualAlerts = append(actualAlerts, id)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.ElementsMatch(t, expectedAlerts, actualAlerts)
	pending, err := repo.Outbox().Pending(ctx, 50)
	require.NoError(t, err)
	actualApplicants := []string{}
	for _, event := range pending {
		var matched eventsv1.JobAlertMatched
		require.NoError(t, event.Data.UnmarshalTo(&matched))
		require.Equal(t, job.JobId, matched.JobId)
		actualApplicants = append(actualApplicants, matched.ApplicantId)
	}
	require.ElementsMatch(t, expectedApplicants, actualApplicants)
	// A malformed nonempty category is permanent and never claims an event.
	e.EventId = uuid.NewString()
	job.CategoryId = "malformed"
	e.Data, err = anypb.New(&job)
	require.NoError(t, err)
	require.ErrorIs(t, handler(ctx, e), events.ErrPermanent)
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id=$1", e.EventId).Scan(&count))
	require.Zero(t, count)
	pending, err = repo.Outbox().Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, len(expectedApplicants))
}
