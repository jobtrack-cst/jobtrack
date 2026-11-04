package events

import (
	"context"
	"errors"
	"strings"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubConfirmation struct {
	ack  bool
	err  error
	wait func(context.Context) (bool, error)
}

func (c stubConfirmation) WaitContext(ctx context.Context) (bool, error) {
	if c.wait != nil {
		return c.wait(ctx)
	}
	return c.ack, c.err
}

type stubChannel struct {
	declareErr, confirmErr, publishErr, closeErr error
	declared, confirmed                          bool
	closes, publishes                            int
	result                                       confirmation
	send                                         func(context.Context, string, string, amqp.Publishing)
	t                                            *testing.T
}

func (ch *stubChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	ch.t.Helper()
	if name != "jobtrack.events" || kind != "topic" || !durable || autoDelete || internal || noWait || args != nil {
		ch.t.Error("incorrect durable topic exchange declaration")
	}
	ch.declared = true
	return ch.declareErr
}

func (ch *stubChannel) Confirm(noWait bool) error {
	if noWait {
		ch.t.Error("confirmation setup must wait for broker")
	}
	ch.confirmed = true
	return ch.confirmErr
}

func (ch *stubChannel) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) (confirmation, error) {
	ch.publishes++
	if ch.send != nil {
		ch.send(ctx, exchange, key, msg)
	}
	return ch.result, ch.publishErr
}

func (ch *stubChannel) Close() error { ch.closes++; return ch.closeErr }

type stubConnection struct {
	ch                   *stubChannel
	channelErr, closeErr error
	closes               int
}

func (c *stubConnection) channel() (publisherChannel, error) { return c.ch, c.channelErr }
func (c *stubConnection) Close() error                       { c.closes++; return c.closeErr }

func publisherEnvelope(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	data, err := anypb.New(&eventsv1.ResumeUploaded{
		ResumeId: "22222222-2222-4222-8222-222222222222", ApplicantId: "33333333-3333-4333-8333-333333333333",
	})
	if err != nil {
		t.Fatal(err)
	}
	return &eventsv1.EventEnvelope{
		EventId: "11111111-1111-4111-8111-111111111111", Type: "resume.uploaded",
		OccurredAt: &timestamppb.Timestamp{Seconds: 1793685600}, Producer: "resume", Data: data,
	}
}

func TestPublisher_PublishesPersistentJSONAndWaitsForAck(t *testing.T) {
	envelope := publisherEnvelope(t)
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
	waited := false
	ch := &stubChannel{t: t, result: stubConfirmation{wait: func(got context.Context) (bool, error) {
		waited = true
		if got != ctx {
			t.Error("confirmation lost caller context")
		}
		return true, nil
	}}}
	ch.send = func(got context.Context, exchange, key string, msg amqp.Publishing) {
		if got != ctx || exchange != "jobtrack.events" || key != envelope.Type {
			t.Error("wrong context, exchange or routing key")
		}
		if msg.ContentType != "application/json" || msg.DeliveryMode != amqp.Persistent {
			t.Error("message must be persistent JSON")
		}
		decoded, err := UnmarshalEnvelope(msg.Body)
		if err != nil || !proto.Equal(decoded, envelope) {
			t.Errorf("serialized envelope changed: %v", err)
		}
	}
	conn := &stubConnection{ch: ch}
	p, err := newPublisher("caller-supplied-url", func(url string) (publisherConnection, error) {
		if url != "caller-supplied-url" {
			t.Error("URL changed")
		}
		return conn, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Publish(ctx, envelope); err != nil {
		t.Fatal(err)
	}
	if !ch.declared || !ch.confirmed || !waited || ch.publishes != 1 {
		t.Fatal("exchange, confirm setup, publish or confirmation missing")
	}
}

func TestPublisher_RejectsInvalidEnvelopeAndCanceledContext(t *testing.T) {
	for _, name := range []string{"nil envelope", "invalid metadata", "canceled context"} {
		t.Run(name, func(t *testing.T) {
			ch := &stubChannel{t: t}
			p := &Publisher{ch: ch}
			ctx := context.Background()
			envelope := publisherEnvelope(t)
			switch name {
			case "nil envelope":
				envelope = nil
			case "invalid metadata":
				envelope.Type = ""
			case "canceled context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := p.Publish(ctx, envelope)
			if err == nil || ch.publishes != 0 {
				t.Fatal("invalid or canceled publish reached channel")
			}
			if name == "canceled context" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestPublisher_ReportsPublishAndConfirmationFailures(t *testing.T) {
	failure := errors.New("broker failure")
	tests := []struct {
		name       string
		publishErr error
		result     confirmation
		want       string
	}{
		{"publish error", failure, nil, "publish event"},
		{"missing confirmation", nil, nil, "confirmation unavailable"},
		{"negative acknowledgement", nil, stubConfirmation{}, "did not acknowledge"},
		{"confirmation channel failure", nil, stubConfirmation{err: failure}, "wait for broker confirmation"},
		{"canceled confirmation", nil, stubConfirmation{wait: func(ctx context.Context) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		}}, "context canceled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch := &stubChannel{t: t, publishErr: tt.publishErr, result: tt.result}
			if tt.name == "canceled confirmation" {
				ch.send = func(context.Context, string, string, amqp.Publishing) { cancel() }
			}
			err := (&Publisher{ch: ch}).Publish(ctx, publisherEnvelope(t))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
			if tt.name == "canceled confirmation" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if tt.publishErr != nil && !errors.Is(err, failure) {
				t.Fatal("lost publish error")
			}
		})
	}
}

func TestNewPublisher_CleansUpAfterInitializationFailure(t *testing.T) {
	failure := errors.New("failure")
	for _, stage := range []string{"URL", "connection", "channel", "exchange", "confirm"} {
		t.Run(stage, func(t *testing.T) {
			ch := &stubChannel{t: t}
			conn := &stubConnection{ch: ch}
			url := "amqp://synthetic-user:synthetic-password@broker/"
			if stage == "URL" {
				url = " "
			}
			if stage == "channel" {
				conn.channelErr = failure
			}
			if stage == "exchange" {
				ch.declareErr = failure
			}
			if stage == "confirm" {
				ch.confirmErr = failure
			}
			dials := 0
			p, err := newPublisher(url, func(string) (publisherConnection, error) {
				dials++
				if stage == "connection" {
					return nil, errors.New(url)
				}
				return conn, nil
			})
			if err == nil || p != nil || strings.Contains(err.Error(), "synthetic-password") {
				t.Fatalf("unexpected initialization result: %v", err)
			}
			wantConn, wantChannel := 1, 1
			if stage == "URL" || stage == "connection" {
				wantConn, wantChannel = 0, 0
			}
			if stage == "channel" {
				wantChannel = 0
			}
			if conn.closes != wantConn || ch.closes != wantChannel {
				t.Fatal("initialization leaked resources")
			}
			if stage == "URL" && dials != 0 {
				t.Fatal("empty URL attempted a connection")
			}
		})
	}
}

func TestPublisher_CloseReleasesBothResourcesOnce(t *testing.T) {
	for _, closeErr := range []error{nil, errors.New("channel close failed"), amqp.ErrClosed} {
		ch := &stubChannel{t: t, closeErr: closeErr}
		conn := &stubConnection{ch: ch}
		p := &Publisher{conn: conn, ch: ch}
		for range 2 {
			err := p.Close()
			if closeErr != nil && closeErr != amqp.ErrClosed && !errors.Is(err, closeErr) {
				t.Fatal("close error lost")
			}
			if (closeErr == nil || closeErr == amqp.ErrClosed) && err != nil {
				t.Fatal(err)
			}
		}
		if ch.closes != 1 || conn.closes != 1 {
			t.Fatal("resources not closed exactly once")
		}
	}
}
