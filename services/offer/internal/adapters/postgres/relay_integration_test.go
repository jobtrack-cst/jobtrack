//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"

	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func startRelayBroker(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "relay_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
			WaitingFor: wait.ForListeningPort("5672/tcp"),
		},
		Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
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
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("relay_test", "synthetic-test-password"), Path: "/"}).String()
}

// A failed mark models a crash after broker confirmation: the same ID is resent.
type failMark struct {
	*postgres.Outbox
	fail bool
}

func (f *failMark) MarkPublished(ctx context.Context, id string, at time.Time) error {
	if f.fail {
		return context.Canceled
	}
	return f.Outbox.MarkPublished(ctx, id, at)
}
func TestRelay_ConfirmedPublishCrashAndRetry(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	url := startRelayBroker(t, ctx)
	pub, err := rabbitmq.NewPublisher(url)
	require.NoError(t, err)
	defer pub.Close()
	conn, err := amqp.Dial(url)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "offer.*", "jobtrack.events", false, nil))
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)
	repo := postgres.NewOffers(pool)
	for _, status := range []domain.OfferStatus{domain.OfferStatusAccepted, domain.OfferStatusDeclined} {
		o := syntheticOffer()
		_, err = repo.Create(ctx, o)
		require.NoError(t, err)
		_, err = repo.Decide(ctx, o.ID, o.ApplicantID, status, o.UpdatedAt.Add(time.Second))
		require.NoError(t, err)
	}
	out := &failMark{Outbox: postgres.NewOutbox(pool), fail: true}
	pending, err := out.Pending(ctx, 50)
	require.NoError(t, err)
	require.Len(t, pending, 4)
	relay := app.NewOutboxRelay(out, pub, fixedClock{time.Now()})
	n, err := relay.PublishPending(ctx)
	require.Error(t, err)
	require.Zero(t, n)
	select {
	case d := <-deliveries:
		e, err := events.UnmarshalEnvelope(d.Body)
		require.NoError(t, err)
		require.True(t, proto.Equal(pending[0], e))
	case <-ctx.Done():
		t.Fatal("missing confirmed delivery")
	}
	out.fail = false
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	for _, want := range pending {
		select {
		case d := <-deliveries:
			e, err := events.UnmarshalEnvelope(d.Body)
			require.NoError(t, err)
			require.True(t, proto.Equal(want, e))
			require.Equal(t, want.Type, d.RoutingKey)
		case <-ctx.Done():
			t.Fatal("missing replayed event")
		}
	}
	n, err = relay.PublishPending(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestApplicationConsumer_RealBrokerAndPostgres(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	url := startRelayBroker(t, ctx)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	ready := make(chan struct{})
	var readyOnce sync.Once
	cfg := rabbitmq.ApplicationConsumerConfig(url, slog.New(slog.NewTextHandler(io.Discard, nil)), postgres.NewParticipants(pool))
	cfg.OnReady = func() { readyOnce.Do(func() { close(ready) }) }
	go func() { done <- events.Consume(workerCtx, cfg) }()
	defer func() { cancel(); require.NoError(t, <-done) }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("application consumer did not become ready", ctx.Err())
	}
	pub, err := rabbitmq.NewPublisher(url)
	require.NoError(t, err)
	defer pub.Close()
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	require.NoError(t, pub.Publish(ctx, e))
	require.NoError(t, pub.Publish(ctx, e))
	require.Eventually(t, func() bool {
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id=$1`, e.EventId).Scan(&n)
		return err == nil && n == 1
	}, 10*time.Second, 50*time.Millisecond)
	p, err := postgres.NewParticipants(pool).GetByApplicationID(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Equal(t, "22222222-2222-4222-8222-222222222222", p.ApplicantID)
}
