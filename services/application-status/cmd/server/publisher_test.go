package main

import (
	"context"
	"errors"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
)

// fakeConnection records what happened to one RabbitMQ connection.
type fakeConnection struct {
	id         int
	publishErr error
	published  int
	closed     int
}

func (c *fakeConnection) Publish(context.Context, *eventsv1.EventEnvelope) error {
	c.published++
	return c.publishErr
}

func (c *fakeConnection) Close() error { c.closed++; return nil }

func TestRecoveringPublisher_ReconnectsAfterAFailedPublish(t *testing.T) {
	errBroker := errors.New("broker restarted")
	first := &fakeConnection{id: 1, publishErr: errBroker}
	var opened []*fakeConnection
	p := newRecoveringPublisher(first, func() (publisherResource, error) {
		c := &fakeConnection{id: 2 + len(opened)}
		opened = append(opened, c)
		return c, nil
	})

	if err := p.Publish(context.Background(), nil); !errors.Is(err, errBroker) {
		t.Fatalf("err=%v, want the broker error", err)
	}
	if first.closed != 1 {
		t.Fatal("the failed connection was not closed")
	}
	if err := p.Publish(context.Background(), nil); err != nil {
		t.Fatalf("publish after reconnect: %v", err)
	}
	if err := p.Publish(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0].published != 2 {
		t.Fatalf("opened %d connections; want one new connection used twice", len(opened))
	}
	if err := p.Close(); err != nil || opened[0].closed != 1 {
		t.Fatal("Close must close the current connection")
	}
	if err := p.Publish(context.Background(), nil); err == nil {
		t.Fatal("publish after Close must fail")
	}
	if err := p.Close(); err != nil {
		t.Fatal("a second Close must be safe")
	}
}

func TestRecoveringPublisher_FailedReconnectIsRetriedNextTime(t *testing.T) {
	first := &fakeConnection{publishErr: errors.New("broker down")}
	attempts := 0
	p := newRecoveringPublisher(first, func() (publisherResource, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("private amqp://user:secret@rabbitmq")
		}
		return &fakeConnection{}, nil
	})
	_ = p.Publish(context.Background(), nil)

	err := p.Publish(context.Background(), nil)
	if err == nil || err.Error() != "reconnect RabbitMQ publisher failed" {
		t.Fatalf("err=%v, want the fixed reconnect message without credentials", err)
	}
	if err := p.Publish(context.Background(), nil); err != nil {
		t.Fatalf("the next publish should reconnect: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}
