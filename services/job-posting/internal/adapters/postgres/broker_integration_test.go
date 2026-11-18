//go:build integration

package postgres_test

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testClock struct{}

func (testClock) Now() time.Time { return time.Now().UTC() }

// startBroker starts a real RabbitMQ, as the job-alert (JT-76) tests do.
func startBroker(ctx context.Context, t *testing.T) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "job_posting_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
			WaitingFor: wait.ForListeningPort("5672/tcp"),
		},
		Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate RabbitMQ test container failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start RabbitMQ test container failed")
	}
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()),
		User: url.UserPassword("job_posting_test", "synthetic-test-password"), Path: "/"}).String()
}

// bindJobClosed returns deliveries of every job.closed message on jobtrack.events,
// through a temporary queue bound before anything is published.
func bindJobClosed(t *testing.T, brokerURL string) <-chan amqp.Delivery {
	t.Helper()
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ch, err := conn.Channel()
	require.NoError(t, err)
	if err := ch.ExchangeDeclarePassive("jobtrack.events", "topic", true, false, false, false, nil); err != nil {
		// A failed passive declare closes the channel, so open a new one.
		ch, err = conn.Channel()
		require.NoError(t, err)
		require.NoError(t, ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil))
	}
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "job.closed", "jobtrack.events", false, nil))
	deliveries, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)
	return deliveries
}

func TestRecordJobRemoved_PublishesOneJobClosedOnJobtrackEvents(t *testing.T) {
	ctx, outbox, jobs := startPostgres(t)
	brokerURL := startBroker(ctx, t)

	publisher, err := rabbitmq.NewPublisher(brokerURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	deliveries := bindJobClosed(t, brokerURL)

	job := createJob(ctx, t, jobs, domain.StatusPublished)
	eventID := uuid.NewString()
	require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, job.ID))
	require.NoError(t, jobs.RecordJobRemoved(ctx, eventID, job.ID))          // redelivery
	require.NoError(t, jobs.RecordJobRemoved(ctx, uuid.NewString(), job.ID)) // already removed

	relay := app.NewOutboxRelay(outbox, publisher, testClock{})
	published, err := relay.PublishPending(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, published)

	select {
	case d := <-deliveries:
		assert.Equal(t, "job.closed", d.RoutingKey)
		env, err := events.UnmarshalEnvelope(d.Body)
		require.NoError(t, err)
		assert.Equal(t, "job.closed", env.Type)
		assert.Equal(t, "job-posting", env.Producer)
		var data eventsv1.JobClosed
		require.NoError(t, env.Data.UnmarshalTo(&data))
		assert.Equal(t, job.ID, data.JobId)
	case <-time.After(10 * time.Second):
		t.Fatal("no job.closed received on jobtrack.events")
	}

	select {
	case d := <-deliveries:
		t.Fatalf("unexpected second message with routing key %q", d.RoutingKey)
	case <-time.After(2 * time.Second):
	}

	assert.Empty(t, closedEvents(ctx, t, outbox, job.ID), "the relay marks the row published")
}
