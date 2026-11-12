//go:build integration

package events

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	consumerTestQueue = "consumer-test.events"
	succeedID         = "a0000000-0000-4000-8000-000000000001"
	permanentID       = "a0000000-0000-4000-8000-000000000002"
	flakyID           = "a0000000-0000-4000-8000-000000000003"
	alwaysFailID      = "a0000000-0000-4000-8000-000000000004"
)

// startConsumerBroker starts RabbitMQ the same way as the publisher test. The
// short confirm timeout lets at-least-once dead-lettering finish quickly.
func startConsumerBroker(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env: map[string]string{
				"RABBITMQ_DEFAULT_USER": "consumer_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password",
				"RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS": "-rabbit dead_letter_worker_publisher_confirm_timeout 1000",
			},
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
	if err != nil {
		t.Fatal("get RabbitMQ host failed")
	}
	port, err := container.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatal("get RabbitMQ port failed")
	}
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("consumer_test", "synthetic-test-password"), Path: "/"}).String()
}

func consumerTestEnvelope(t *testing.T, eventID string) *eventsv1.EventEnvelope {
	t.Helper()
	data, err := anypb.New(&eventsv1.EmployerVerified{EmployerId: "55555555-5555-4555-8555-555555555555"})
	require.NoError(t, err)
	return &eventsv1.EventEnvelope{
		EventId: eventID, Type: "employer.verified", OccurredAt: timestamppb.Now(),
		Producer: "company-verification", Data: data,
	}
}

// deadEventIDs reads the dead queue until it holds want messages or the
// deadline passes. Malformed bodies are returned as "malformed".
func deadEventIDs(t *testing.T, ctx context.Context, ch *amqp.Channel, want int) []string {
	t.Helper()
	var ids []string
	for len(ids) < want {
		msg, ok, err := ch.Get(consumerTestQueue+".dead", true)
		require.NoError(t, err)
		if !ok {
			select {
			case <-ctx.Done():
				t.Fatalf("dead queue has %d of %d expected messages", len(ids), want)
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		envelope, err := UnmarshalEnvelope(msg.Body)
		if err != nil {
			ids = append(ids, "malformed")
			continue
		}
		ids = append(ids, envelope.GetEventId())
	}
	return ids
}

func TestConsume_IntegrationAcksRetriesAndDeadLetters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	brokerURL := startConsumerBroker(t, ctx)

	var mu sync.Mutex
	calls := map[string]int{}
	handled := make(chan string, 20)
	handler := func(_ context.Context, e *eventsv1.EventEnvelope) error {
		mu.Lock()
		calls[e.GetEventId()]++
		n := calls[e.GetEventId()]
		mu.Unlock()
		handled <- e.GetEventId()
		switch e.GetEventId() {
		case permanentID:
			return Permanent(io.ErrUnexpectedEOF)
		case flakyID:
			if n < 3 {
				return io.ErrUnexpectedEOF
			}
		case alwaysFailID:
			return io.ErrUnexpectedEOF
		}
		return nil
	}

	ready := make(chan struct{}, 1)
	consumeCtx, stopConsumer := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Consume(consumeCtx, ConsumerConfig{
			URL: brokerURL, Queue: consumerTestQueue, Bindings: []string{"employer.verified"},
			Handler: handler, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			OnReady:   func() { ready <- struct{}{} },
			retryBase: 10 * time.Millisecond,
		})
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("consumer did not become ready")
	}

	publisher, err := NewPublisher(brokerURL)
	require.NoError(t, err)
	defer publisher.Close()
	for _, id := range []string{succeedID, permanentID, flakyID, alwaysFailID} {
		require.NoError(t, publisher.Publish(ctx, consumerTestEnvelope(t, id)))
	}

	// A body that is not an envelope, published straight to the exchange.
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.PublishWithContext(ctx, eventExchange, "employer.verified", false, false,
		amqp.Publishing{ContentType: "application/json", Body: []byte(`{"broken":true}`)}))

	dead := deadEventIDs(t, ctx, ch, 3)
	require.ElementsMatch(t, []string{permanentID, alwaysFailID, "malformed"}, dead)

	// Wait until the flaky event has had its third, successful attempt.
	for {
		mu.Lock()
		n := calls[flakyID]
		mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-handled:
		case <-ctx.Done():
			t.Fatal("flaky event was not retried three times")
		}
	}

	mu.Lock()
	require.Equal(t, 1, calls[succeedID], "a successful event is handled once")
	require.Equal(t, 1, calls[permanentID], "a permanent failure is not retried")
	require.Equal(t, 3, calls[flakyID], "a transient failure is retried until it succeeds")
	require.Equal(t, MaxDeliveries, calls[alwaysFailID], "a failing event gets MaxDeliveries attempts")
	mu.Unlock()

	stopConsumer()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Consume did not stop after cancellation")
	}

	// Nothing is left behind: the succeeded and retried events were acked.
	q, err := ch.QueueDeclarePassive(consumerTestQueue, true, false, false, false, sourceQueueArgs(consumerTestQueue))
	require.NoError(t, err)
	require.Zero(t, q.Messages)
}
