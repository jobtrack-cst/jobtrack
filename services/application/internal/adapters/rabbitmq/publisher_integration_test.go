//go:build integration

package rabbitmq

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
)

func TestPublisher_IntegrationRoutesEnvelopeToBoundQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "publisher_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
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
	brokerURL := (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("publisher_test", "synthetic-test-password"), Path: "/"}).String()
	publisher, err := NewPublisher(brokerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			t.Error(err)
		}
	}()
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		t.Fatal("open test queue connection failed")
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal("open test queue channel failed")
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal("declare test queue failed")
	}
	if err := ch.QueueBind(queue.Name, "application.*", "jobtrack.events", false, nil); err != nil {
		t.Fatal("bind test queue failed")
	}
	deliveries, err := ch.ConsumeWithContext(ctx, queue.Name, "", false, true, false, false, nil)
	if err != nil {
		t.Fatal("consume test queue failed")
	}
	for _, key := range []string{"application.submitted", "application.withdrawn"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", key+".json"))
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := events.UnmarshalEnvelope(raw)
		if err != nil {
			t.Fatal(err)
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		if err := publisher.Publish(canceled, envelope); err == nil {
			t.Fatal("canceled publish succeeded")
		}

		if err := publisher.Publish(ctx, envelope); err != nil {
			t.Fatal(err)
		}
		select {
		case delivery, ok := <-deliveries:
			if !ok {
				t.Fatal("test queue closed before delivery")
			}
			if delivery.Exchange != "jobtrack.events" || delivery.RoutingKey != envelope.Type || delivery.ContentType != "application/json" || delivery.DeliveryMode != amqp.Persistent {
				t.Fatal("delivery routing or properties changed")
			}
			decoded, err := events.UnmarshalEnvelope(delivery.Body)
			if err != nil || !proto.Equal(decoded, envelope) {
				t.Fatalf("envelope did not round-trip: %v", err)
			}
			if err := delivery.Ack(false); err != nil {
				t.Fatal("acknowledge test delivery failed")
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for routed event")
		}

	}
}
