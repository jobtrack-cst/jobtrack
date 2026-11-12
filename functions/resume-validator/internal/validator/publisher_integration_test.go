//go:build integration

package validator_test

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestBrokerPublisher_IntegrationBoundsStalledLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"}, Env: map[string]string{"RABBITMQ_DEFAULT_USER": "validator_test", "RABBITMQ_DEFAULT_PASS": "synthetic-password"}, WaitingFor: wait.ForListeningPort("5672/tcp")}, Started: true})
	if container != nil {
		t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	}
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)
	target := net.JoinHostPort(host, port.Port())
	broker := (&url.URL{Scheme: "amqp", Host: target, User: url.UserPassword("validator_test", "synthetic-password"), Path: "/"}).String()
	fixture, err := os.ReadFile("../../../../tests/contract/fixtures/resume.validated.json")
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(fixture)
	require.NoError(t, err)
	observer, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer observer.Close()
	ch, err := observer.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil))
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(queue.Name, "resume.validated", "jobtrack.events", false, nil))
	deliveries, err := ch.ConsumeWithContext(ctx, queue.Name, "", true, true, false, false, nil)
	require.NoError(t, err)
	for _, tc := range []struct {
		name          string
		class, method uint16
		failure       bool
	}{
		{"channel open", 20, 11, true},
		{"exchange declare", 40, 11, true},
		{"confirm select", 85, 11, true},
		{"publish confirmation", 60, 80, true},
		{"connection close", 10, 51, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address, blocked, finished := stallBrokerReply(t, target, tc.class, tc.method)
			proxyURL := (&url.URL{Scheme: "amqp", Host: address, User: url.UserPassword("validator_test", "synthetic-password"), Path: "/"}).String()
			callCtx, callCancel := context.WithTimeout(ctx, 300*time.Millisecond)
			defer callCancel()
			started := time.Now()
			err := (validator.BrokerPublisher{URL: proxyURL}).Publish(callCtx, envelope)
			if tc.failure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Less(t, time.Since(started), 2*time.Second, "publisher escaped context/cleanup budget")
			select {
			case <-blocked:
			case <-time.After(time.Second):
				t.Fatal("test did not stall the requested broker reply")
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("publisher left a live proxy connection")
			}
			if tc.class == 60 || tc.class == 10 {
				// The broker got the event even if its confirm was lost. Retry the SAME
				// envelope; downstream deduplication must see the SAME event ID.
				select {
				case delivery := <-deliveries:
					decoded, err := events.UnmarshalEnvelope(delivery.Body)
					require.NoError(t, err)
					require.Equal(t, envelope.EventId, decoded.EventId)
				case <-time.After(time.Second):
					t.Fatal("confirmed/uncertain publication never reached broker queue")
				}
				require.NoError(t, (validator.BrokerPublisher{URL: broker}).Publish(ctx, envelope))
				select {
				case delivery := <-deliveries:
					decoded, err := events.UnmarshalEnvelope(delivery.Body)
					require.NoError(t, err)
					require.Equal(t, envelope.EventId, decoded.EventId)
				case <-time.After(time.Second):
					t.Fatal("retry did not publish the same event identity")
				}
			}
		})
	}
}

// This TCP proxy forwards a real AMQP session and withholds one selected method
// reply. It does not invent confirmations or replace RabbitMQ with a fake broker.
func stallBrokerReply(t *testing.T, target string, class, method uint16) (string, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	halted := make(chan struct{})
	done := make(chan struct{})
	stop := make(chan struct{})
	var peers sync.Mutex
	var downstream, upstream net.Conn
	t.Cleanup(func() {
		close(stop)
		_ = listener.Close()
		peers.Lock()
		defer peers.Unlock()
		if downstream != nil {
			_ = downstream.Close()
		}
		if upstream != nil {
			_ = upstream.Close()
		}
	})
	go func() {
		defer close(done)
		client, err := listener.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		server, err := net.DialTimeout("tcp", target, time.Second)
		if err != nil {
			return
		}
		defer server.Close()
		peers.Lock()
		downstream = client
		upstream = server
		peers.Unlock()
		sent := make(chan struct{})
		go func() { _, _ = io.Copy(server, client); close(sent) }()
		defer func() { _ = client.Close(); _ = server.Close(); <-sent }()
		for {
			var header [7]byte
			if _, err := io.ReadFull(server, header[:]); err != nil {
				return
			}
			length := binary.BigEndian.Uint32(header[3:7])
			if length > 1024*1024 {
				return
			}
			payload := make([]byte, int(length)+1)
			if _, err := io.ReadFull(server, payload); err != nil {
				return
			}
			if header[0] == 1 && length >= 4 && binary.BigEndian.Uint16(payload[:2]) == class && binary.BigEndian.Uint16(payload[2:4]) == method {
				close(halted)
				select {
				case <-sent:
				case <-stop:
				}
				return
			}
			if _, err := client.Write(header[:]); err != nil {
				return
			}
			if _, err := client.Write(payload); err != nil {
				return
			}
		}
	}()
	return listener.Addr().String(), halted, done
}
