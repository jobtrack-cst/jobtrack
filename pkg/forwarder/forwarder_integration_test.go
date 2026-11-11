//go:build integration

package forwarder_test

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestForwarder_IntegrationDeliveryAndRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	conn, err := amqp.Dial(brokerURL)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.Confirm(false))
	for _, scenario := range []string{"success", "poison", "exhaustion", "restart", "crash-budget", "missing-dlq"} {
		t.Run(scenario, func(t *testing.T) {
			function := "test-" + scenario
			queue := function + ".events"
			var calls atomic.Int32
			entered := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if (scenario == "restart" && n == 1) || scenario == "crash-budget" {
					entered <- struct{}{}
					_, _ = io.Copy(io.Discard, r.Body)
					<-r.Context().Done()
					return
				}
				if scenario == "success" {
					w.WriteHeader(204)
				} else {
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			start := func() (context.CancelFunc, chan error) {
				runCtx, cancelRun := context.WithCancel(ctx)
				done := make(chan error, 1)
				go func() {
					done <- forwarder.Run(runCtx, forwarder.Config{Broker: brokerURL, Sink: server.URL, Function: function, Bindings: []string{"resume.uploaded"}}, logger)
				}()
				require.Eventually(t, func() bool {
					probe, e := conn.Channel()
					if e != nil {
						return false
					}
					defer probe.Close()
					q, e := probe.QueueDeclarePassive(queue, true, false, false, false, nil)
					return e == nil && q.Consumers == 1
				}, 5*time.Second, 20*time.Millisecond)
				return cancelRun, done
			}
			cancelRun, done := start()
			defer func() {
				cancelRun()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("shutdown did not finish")
				}
			}()
			if scenario == "missing-dlq" {
				_, e := ch.QueueDelete(queue+".dead", false, false, false)
				require.NoError(t, e)
			}
			body := envelope(t)
			if scenario == "poison" {
				body = []byte("malformed")
			}
			confirmation, e := ch.PublishWithDeferredConfirmWithContext(ctx, "jobtrack.events", "resume.uploaded", false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: body, ContentType: "application/json"})
			require.NoError(t, e)
			ok, e := confirmation.WaitContext(ctx)
			require.NoError(t, e)
			require.True(t, ok)
			if scenario == "restart" {
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				cancelRun()
				require.NoError(t, <-done)
				cancelRun, done = start()
			}
			if scenario == "crash-budget" {
				for i := 0; i < 4; i++ {
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					cancelRun()
					require.NoError(t, <-done)
					cancelRun, done = start()
				}
			}
			if scenario == "missing-dlq" {
				require.Eventually(t, func() bool { return calls.Load() == 4 }, 15*time.Second, 20*time.Millisecond)
				// Retained dead letters are not included in basic.declare's ready count.
				// Leave the target absent while the internal DLX worker tries to transfer.
				time.Sleep(250 * time.Millisecond)
				_, available, e := ch.Get(queue, true)
				require.NoError(t, e)
				require.False(t, available)

				_, e = ch.QueueDeclare(queue+".dead", true, false, false, false, amqp.Table{"x-queue-type": "quorum"})
				require.NoError(t, e)
			}
			if scenario == "success" {
				require.Eventually(t, func() bool { return calls.Load() == 1 }, 3*time.Second, 20*time.Millisecond)
				// Stopping the consumer returns unacked messages; Get must still be empty.
				time.Sleep(100 * time.Millisecond)
				cancelRun()
				require.NoError(t, <-done)
				done = make(chan error, 1)
				done <- nil
				_, ok, e := ch.Get(queue, true)
				require.NoError(t, e)
				require.False(t, ok)
			} else {
				require.Eventually(t, func() bool {
					msg, ok, e := ch.Get(queue+".dead", true)
					if e != nil {
						return false
					}
					if ok {
						require.Equal(t, body, msg.Body)
					}
					return ok
				}, 25*time.Second, 50*time.Millisecond)
				if scenario == "poison" {
					require.Zero(t, calls.Load())
				} else {
					require.EqualValues(t, 4, calls.Load())
				}
			}
		})
	}
}
