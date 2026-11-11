package forwarder_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type settlement struct {
	ack, nack, retry int
	err              error
}

func (s *settlement) Ack(multiple bool) error {
	if multiple {
		panic("multiple ack")
	}
	s.ack++
	return s.err
}
func (s *settlement) Nack(multiple, requeue bool) error {
	if multiple || requeue {
		panic("unsafe nack")
	}
	s.nack++
	return s.err
}
func (s *settlement) Reject(requeue bool) error {
	if !requeue {
		panic("unexpected reject")
	}
	s.retry++
	return s.err
}
func envelope(t *testing.T) []byte {
	t.Helper()
	data, err := anypb.New(&eventsv1.ResumeUploaded{ResumeId: "22222222-2222-4222-8222-222222222222"})
	require.NoError(t, err)
	body, err := events.MarshalEnvelope(&eventsv1.EventEnvelope{EventId: "11111111-1111-4111-8111-111111111111", Type: "resume.uploaded", OccurredAt: timestamppb.Now(), Producer: "resume", Data: data})
	require.NoError(t, err)
	return append([]byte(" \n"), append(body, []byte("\n ")...)...)
}
func TestHandler_MappingAndSettlement(t *testing.T) {
	for _, status := range []int{200, 204, 299, 300, 400, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			body := envelope(t)
			var received []byte
			var headers http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers = r.Header.Clone()
				received, _ = io.ReadAll(r.Body)
				w.WriteHeader(status)
			}))
			defer server.Close()
			s := &settlement{}
			h := forwarder.NewHandler(server.URL, nil)
			require.NoError(t, h.Handle(context.Background(), body, nil, s))
			require.Equal(t, body, received)
			require.Equal(t, "application/json", headers.Get("Content-Type"))
			require.Equal(t, "1.0", headers.Get("Ce-Specversion"))
			require.Equal(t, "11111111-1111-4111-8111-111111111111", headers.Get("Ce-Id"))
			require.Equal(t, "resume.uploaded", headers.Get("Ce-Type"))
			require.Equal(t, "/jobtrack/rabbitmq", headers.Get("Ce-Source"))
			if status < 300 {
				require.Equal(t, 1, s.ack)
			} else {
				require.Equal(t, 1, s.retry)
			}
			require.Zero(t, s.nack)
		})
	}
}
func TestHandler_RestartPreservesBrokerCountAndExhausts(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	body := envelope(t)
	for count := int64(0); count < 4; count++ {
		// Fresh handler simulates a new process; state arrives with broker delivery.
		h := forwarder.NewHandler(server.URL, nil)
		s := &settlement{}
		require.NoError(t, h.Handle(context.Background(), body, amqp.Table{"x-delivery-count": count}, s))
		require.Zero(t, s.ack)
		if count < 3 {
			require.Equal(t, 1, s.retry)
		} else {
			require.Equal(t, 1, s.nack)
		}
	}
	require.EqualValues(t, 4, calls.Load())
	s := &settlement{}
	require.NoError(t, forwarder.NewHandler(server.URL, nil).Handle(context.Background(), body, amqp.Table{"x-delivery-count": int64(4)}, s))
	require.Equal(t, 1, s.nack)
	require.EqualValues(t, 4, calls.Load())
}
func TestHandler_FailureThenSuccess(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	body := envelope(t)
	s := &settlement{}
	require.NoError(t, forwarder.NewHandler(server.URL, nil).Handle(context.Background(), body, nil, s))
	require.Equal(t, 1, s.retry)
	s = &settlement{}
	require.NoError(t, forwarder.NewHandler(server.URL, nil).Handle(context.Background(), body, amqp.Table{"x-delivery-count": int64(1)}, s))
	require.Equal(t, 1, s.ack)
}
func TestHandler_TimeoutRefusalRedirectAndPoison(t *testing.T) {
	t.Parallel()
	body := envelope(t)
	timeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer timeout.Close()
	h := forwarder.NewHandler(timeout.URL, nil)
	h.Client.Timeout = 20 * time.Millisecond
	s := &settlement{}
	require.NoError(t, h.Handle(context.Background(), body, nil, s))
	require.Equal(t, 1, s.retry)
	require.Zero(t, s.ack)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	s = &settlement{}
	require.NoError(t, forwarder.NewHandler("http://"+addr, nil).Handle(context.Background(), body, nil, s))
	require.Equal(t, 1, s.retry)
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Store(true) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer redirect.Close()
	s = &settlement{}
	require.NoError(t, forwarder.NewHandler(redirect.URL, nil).Handle(context.Background(), body, nil, s))
	require.False(t, followed.Load())
	require.Equal(t, 1, s.retry)
	for _, bad := range [][]byte{[]byte("bad"), []byte(`{}`)} {
		s = &settlement{}
		require.NoError(t, h.Handle(context.Background(), bad, nil, s))
		require.Equal(t, 1, s.nack)
		require.Zero(t, s.ack)
		require.Zero(t, s.retry)
	}
}
func TestHandler_CancellationLeavesUnsettled(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "backoff", true: "active request"}[active], func(t *testing.T) {
			t.Parallel()
			entered := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				_, _ = io.Copy(io.Discard, r.Body)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &settlement{}
			done := make(chan error, 1)
			headers := amqp.Table{}
			if !active {
				headers["x-delivery-count"] = int64(1)
			}
			body := envelope(t)
			go func() { done <- forwarder.NewHandler(server.URL, nil).Handle(ctx, body, headers, s) }()
			if active {
				<-entered
			} else {
				time.Sleep(20 * time.Millisecond)
			}
			cancel()
			require.ErrorIs(t, <-done, context.Canceled)
			require.Equal(t, &settlement{}, s)
		})
	}
}
func TestHandler_SettlementErrorsPropagate(t *testing.T) {
	for _, status := range []int{200, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			expected := errors.New("AMQP settlement failed")
			s := &settlement{err: expected}
			h := forwarder.NewHandler(server.URL, nil)
			require.ErrorIs(t, h.Handle(context.Background(), envelope(t), nil, s), expected)
			s = &settlement{err: expected}
			require.ErrorIs(t, h.Handle(context.Background(), []byte("bad"), nil, s), expected)
			require.Equal(t, 1, s.nack)
		})
	}
}
func TestDial_CancelsStalledHandshake(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := l.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := forwarder.Dial(ctx, "amqp://guest:guest@"+l.Addr().String()+"/"); done <- err }()
	socket := <-accepted
	defer socket.Close()
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("handshake did not cancel")
	}
}

// Ensure fixtures differ bytewise from a canonical re-encoding.
func TestEnvelopeFixture_HasWhitespace(t *testing.T) {
	require.True(t, bytes.HasPrefix(envelope(t), []byte(" \n")))
}
