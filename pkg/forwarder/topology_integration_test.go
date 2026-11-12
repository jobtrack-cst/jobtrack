//go:build integration

package forwarder_test

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func topologyBroker(t *testing.T) (context.Context, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "publisher_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password", "RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS": "-rabbit dead_letter_worker_publisher_confirm_timeout 1000"},
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

	return ctx, brokerURL
}

func TestTopology_IntegrationIndependentProvisioning(t *testing.T) {
	ctx, broker := topologyBroker(t)
	topology := forwarder.ResumeValidatorTopology()
	// Only provision: no forwarder, HTTP sink or validator is started.
	require.NoError(t, forwarder.EnsureTopology(ctx, broker, topology))
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	for _, queue := range []string{"resume-validator.events", "resume-validator.events.dead"} {
		q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
		require.NoError(t, err)
		require.Zero(t, q.Consumers)
	}
	require.NoError(t, ch.Confirm(false))
	body := envelope(t)
	// Routing to source plus retained DLQ contents survive repeated provisioning.
	for _, route := range []struct{ exchange, key string }{{"jobtrack.events", "resume.uploaded"}, {"", "resume-validator.events.dead"}} {
		confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, route.exchange, route.key, true, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: body, ContentType: "application/json"})
		require.NoError(t, err)
		ok, err := confirm.WaitContext(ctx)
		require.NoError(t, err)
		require.True(t, ok)
	}
	require.NoError(t, forwarder.EnsureTopology(ctx, broker, topology))
	require.NoError(t, forwarder.EnsureTopology(ctx, broker, topology))
	for _, queue := range []string{"resume-validator.events", "resume-validator.events.dead"} {
		q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
		require.NoError(t, err)
		require.Equal(t, 1, q.Messages)
		require.Zero(t, q.Consumers)
		msg, ok, err := ch.Get(queue, false)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, body, msg.Body)
		require.NoError(t, msg.Ack(false))
	}
	// An already canceled provisioning operation changes nothing and returns promptly.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, forwarder.EnsureTopology(canceled, broker, topology), context.Canceled)
}

func TestTopology_IntegrationIncompatibleQueuesRetainMessages(t *testing.T) {
	ctx, broker := topologyBroker(t)
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	for _, scenario := range []string{"classic-source", "classic-dead", "wrong-arguments"} {
		t.Run(scenario, func(t *testing.T) {
			ch, err := conn.Channel()
			require.NoError(t, err)
			defer ch.Close()
			topology := forwarder.Topology{Function: "test-" + scenario, Bindings: []string{"resume.uploaded"}}
			queue := topology.Function + ".events"
			args := amqp.Table{"x-queue-type": "classic"}
			if scenario == "classic-dead" {
				queue += ".dead"
			}
			if scenario == "wrong-arguments" {
				args = amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int32(99)}
			}
			_, err = ch.QueueDeclare(queue, true, false, false, false, args)
			require.NoError(t, err)
			require.NoError(t, ch.Confirm(false))
			confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: []byte("retained synthetic event")})
			require.NoError(t, err)
			ok, err := confirm.WaitContext(ctx)
			require.NoError(t, err)
			require.True(t, ok)
			err = forwarder.EnsureTopology(ctx, broker, topology)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "synthetic-test-password")
			require.Contains(t, err.Error(), "reviewed topology migration")
			msg, ok, err := ch.Get(queue, false)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, []byte("retained synthetic event"), msg.Body)
			require.NoError(t, msg.Ack(false))
		})
	}
}

func TestTopology_IntegrationIncompatibleExchange(t *testing.T) {
	ctx, broker := topologyBroker(t)
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.ExchangeDeclare("jobtrack.events", "direct", true, false, false, false, nil))
	err = forwarder.EnsureTopology(ctx, broker, forwarder.ResumeValidatorTopology())
	require.ErrorContains(t, err, "topology compatibility")
	// The original exchange is still usable with its original type.
	require.NoError(t, ch.ExchangeDeclare("jobtrack.events", "direct", true, false, false, false, nil))
}
