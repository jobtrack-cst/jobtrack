package validator_test

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
)

func TestBrokerPublisher_CancelsStalledHandshake(t *testing.T) {
	fixture, err := os.ReadFile("../../../../tests/contract/fixtures/resume.validated.json")
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(fixture)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	peerClosed := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
		close(peerClosed)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = (validator.BrokerPublisher{URL: "amqp://synthetic:password@" + listener.Addr().String() + "/"}).Publish(ctx, envelope)
	require.Error(t, err)
	require.Less(t, time.Since(started), time.Second)
	select {
	case <-peerClosed:
	case <-time.After(time.Second):
		t.Fatal("stalled handshake left its socket open")
	}
}

func TestBrokerPublisher_ExpiredContextDoesNotDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, (validator.BrokerPublisher{URL: "not a broker"}).Publish(ctx, nil), context.Canceled)
}
