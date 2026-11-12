package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type stubAck struct {
	acks, rejects int
	nacks         int
	nackRequeue   bool
	rejectRequeue bool
	err           error
}

func (a *stubAck) Ack(bool) error                                { a.acks++; return a.err }
func (a *stubAck) Nack(_ bool, rq bool) error                    { a.nacks++; a.nackRequeue = rq; return a.err }
func (a *stubAck) Reject(rq bool) error                          { a.rejects++; a.rejectRequeue = rq; return a.err }
func (a *stubAck) settled() int                                  { return a.acks + a.nacks + a.rejects }
func discardLogger() *slog.Logger                                { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func noopHandler(context.Context, *eventsv1.EventEnvelope) error { return nil }

func validConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		URL: "amqp://broker", Queue: "employer.verification",
		Bindings: []string{"employer.verified", "employer.verification_rejected"},
		Handler:  noopHandler, Logger: discardLogger(),
	}
}

func TestConsumerConfig_Validate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ConsumerConfig)
		valid  bool
	}{
		{"valid", func(*ConsumerConfig) {}, true},
		{"kebab-case service and purpose", func(c *ConsumerConfig) { c.Queue = "application-status.status-events" }, true},
		{"empty URL", func(c *ConsumerConfig) { c.URL = " " }, false},
		{"queue without purpose", func(c *ConsumerConfig) { c.Queue = "employer" }, false},
		{"queue with upper case", func(c *ConsumerConfig) { c.Queue = "Employer.verification" }, false},
		{"queue with empty purpose", func(c *ConsumerConfig) { c.Queue = "employer." }, false},
		{"queue named like a dead queue", func(c *ConsumerConfig) { c.Queue = "employer.verification.dead" }, false},
		{"no bindings", func(c *ConsumerConfig) { c.Bindings = nil }, false},
		{"wildcard binding", func(c *ConsumerConfig) { c.Bindings = []string{"employer.*"} }, false},
		{"binding without dot", func(c *ConsumerConfig) { c.Bindings = []string{"employer"} }, false},
		{"nil handler", func(c *ConsumerConfig) { c.Handler = nil }, false},
		{"nil logger", func(c *ConsumerConfig) { c.Logger = nil }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConsumerConfig()
			tt.change(&cfg)
			err := cfg.validate()
			if tt.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestConsumerConfig_Validate_FillsJT61Timings(t *testing.T) {
	cfg := validConsumerConfig()
	require.NoError(t, cfg.validate())
	require.Equal(t, time.Second, cfg.retryBase)
	require.Equal(t, HandlerTimeout, cfg.handlerTimeout)
	require.Equal(t, time.Second, cfg.reconnectBase)
	require.Equal(t, 30*time.Second, cfg.reconnectMax)
}

func TestSourceQueueArgs_MatchJT61Policy(t *testing.T) {
	require.Equal(t, amqp.Table{
		"x-queue-type":              "quorum",
		"x-delivery-limit":          int32(3),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": "employer.verification.dead",
		"x-dead-letter-strategy":    "at-least-once",
		"x-overflow":                "reject-publish",
	}, sourceQueueArgs("employer.verification"))
}

func TestSettle(t *testing.T) {
	transient := errors.New("database unavailable")
	tests := []struct {
		name        string
		body        func(t *testing.T) []byte
		headers     amqp.Table
		handlerErr  error
		cancel      bool
		wantCalls   int
		wantAck     bool
		wantDead    bool
		wantRequeue bool
	}{
		{name: "success is acknowledged", wantCalls: 1, wantAck: true},
		{name: "first delivery has no count header", headers: amqp.Table{}, wantCalls: 1, wantAck: true},
		{name: "malformed body is dead-lettered without calling the handler",
			body: func(*testing.T) []byte { return []byte(`{"not":"an envelope"}`) }, wantDead: true},
		{name: "invalid count header is dead-lettered", headers: amqp.Table{"x-delivery-count": "two"}, wantDead: true},
		{name: "negative count header is dead-lettered", headers: amqp.Table{"x-delivery-count": int64(-1)}, wantDead: true},
		{name: "permanent error is dead-lettered", handlerErr: Permanent(errors.New("bad employer_id")), wantCalls: 1, wantDead: true},
		{name: "transient error on first attempt is retried", handlerErr: transient, wantCalls: 1, wantRequeue: true},
		{name: "transient error on third attempt is retried",
			headers: amqp.Table{"x-delivery-count": int64(2)}, handlerErr: transient, wantCalls: 1, wantRequeue: true},
		{name: "transient error on fourth attempt is dead-lettered",
			headers: amqp.Table{"x-delivery-count": int64(3)}, handlerErr: transient, wantCalls: 1, wantDead: true},
		{name: "redelivery succeeds after the backoff",
			headers: amqp.Table{"x-delivery-count": int64(1)}, wantCalls: 1, wantAck: true},
		{name: "shutdown during the backoff requeues without calling the handler",
			headers: amqp.Table{"x-delivery-count": int64(1)}, cancel: true, wantRequeue: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := publisherEnvelope(t)
			body, err := MarshalEnvelope(envelope)
			require.NoError(t, err)
			if tt.body != nil {
				body = tt.body(t)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			calls := 0
			cfg := validConsumerConfig()
			cfg.Handler = func(_ context.Context, got *eventsv1.EventEnvelope) error {
				calls++
				require.True(t, proto.Equal(envelope, got), "handler must get the decoded envelope")
				return tt.handlerErr
			}
			require.NoError(t, cfg.validate())
			cfg.retryBase = time.Millisecond
			ack := &stubAck{}

			require.NoError(t, settle(ctx, &cfg, body, tt.headers, ack))

			require.Equal(t, tt.wantCalls, calls)
			require.Equal(t, 1, ack.settled(), "every delivery is settled exactly once")
			switch {
			case tt.wantAck:
				require.Equal(t, 1, ack.acks)
			case tt.wantDead:
				require.Equal(t, 1, ack.nacks)
				require.False(t, ack.nackRequeue, "dead-lettering must not requeue")
			case tt.wantRequeue:
				require.Equal(t, 1, ack.rejects)
				require.True(t, ack.rejectRequeue)
			}
		})
	}
}

func TestSettle_HandlerErrorDuringShutdownRequeues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := validConsumerConfig()
	cfg.Handler = func(context.Context, *eventsv1.EventEnvelope) error {
		cancel()
		return context.Canceled
	}
	require.NoError(t, cfg.validate())
	body, err := MarshalEnvelope(publisherEnvelope(t))
	require.NoError(t, err)
	ack := &stubAck{}

	require.NoError(t, settle(ctx, &cfg, body, nil, ack))
	require.Equal(t, 1, ack.rejects)
	require.True(t, ack.rejectRequeue)
}

func TestSettle_ReturnsSettlementError(t *testing.T) {
	cfg := validConsumerConfig()
	require.NoError(t, cfg.validate())
	body, err := MarshalEnvelope(publisherEnvelope(t))
	require.NoError(t, err)
	ack := &stubAck{err: amqp.ErrClosed}

	require.ErrorIs(t, settle(context.Background(), &cfg, body, nil, ack), amqp.ErrClosed)
}

func TestPermanent_KeepsBothErrors(t *testing.T) {
	cause := errors.New("bad employer_id")
	err := Permanent(cause)
	require.ErrorIs(t, err, ErrPermanent)
	require.ErrorIs(t, err, cause)
}

func TestConsume_RejectsInvalidConfig(t *testing.T) {
	cfg := validConsumerConfig()
	cfg.Queue = "employer"
	require.Error(t, Consume(context.Background(), cfg))
}

func TestConsume_StopsReconnectingWhenContextEnds(t *testing.T) {
	cfg := validConsumerConfig()
	// Port 1 refuses connections, so every session fails at dial.
	cfg.URL = "amqp://guest:guest@127.0.0.1:1/"
	cfg.reconnectBase, cfg.reconnectMax = time.Millisecond, 5*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- Consume(ctx, cfg) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Consume did not return after the context ended")
	}
}

func TestSettle_HandlerTimeoutRetriesTheEvent(t *testing.T) {
	cfg := validConsumerConfig()
	require.NoError(t, cfg.validate())
	cfg.handlerTimeout = 20 * time.Millisecond
	cfg.Handler = func(ctx context.Context, _ *eventsv1.EventEnvelope) error {
		_, hasDeadline := ctx.Deadline()
		require.True(t, hasDeadline, "every handler call has a deadline")
		<-ctx.Done() // a database call that never answers
		return ctx.Err()
	}
	body, err := MarshalEnvelope(publisherEnvelope(t))
	require.NoError(t, err)
	ack := &stubAck{}

	require.NoError(t, settle(context.Background(), &cfg, body, nil, ack))
	require.Equal(t, 1, ack.rejects, "a timed-out event is retried, not dead-lettered")
	require.True(t, ack.rejectRequeue)
}
