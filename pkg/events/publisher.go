package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	amqp "github.com/rabbitmq/amqp091-go"
)

const eventExchange = "jobtrack.events"

type confirmation interface {
	WaitContext(context.Context) (bool, error)
}

type publisherChannel interface {
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	Confirm(bool) error
	publish(context.Context, string, string, amqp.Publishing) (confirmation, error)
	Close() error
}

type publisherConnection interface {
	channel() (publisherChannel, error)
	Close() error
}

// Publisher owns its RabbitMQ connection and channel. Close it during shutdown.
type Publisher struct {
	conn      publisherConnection
	ch        publisherChannel
	closeOnce sync.Once
	closeErr  error
}

// NewPublisher connects using the caller's RABBITMQ_URL and declares the exchange.
func NewPublisher(rabbitMQURL string) (*Publisher, error) {
	return newPublisher(rabbitMQURL, func(url string) (publisherConnection, error) {
		conn, err := amqp.Dial(url)
		if err != nil {
			return nil, err
		}
		return rabbitConnection{conn}, nil
	})
}

func newPublisher(url string, dial func(string) (publisherConnection, error)) (*Publisher, error) {
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("RABBITMQ_URL is required")
	}
	conn, err := dial(url)
	if err != nil {
		// Dial errors may contain the URL, including credentials.
		return nil, errors.New("connect to RabbitMQ failed; check RABBITMQ_URL and broker availability")
	}
	p := &Publisher{conn: conn}
	ch, err := conn.channel()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open RabbitMQ channel: %w", err), p.Close())
	}
	p.ch = ch
	if err := ch.ExchangeDeclare(eventExchange, "topic", true, false, false, false, nil); err != nil {
		return nil, errors.Join(fmt.Errorf("declare event exchange: %w", err), p.Close())
	}
	if err := ch.Confirm(false); err != nil {
		return nil, errors.Join(fmt.Errorf("enable publisher confirmations: %w", err), p.Close())
	}
	return p, nil
}

// Publish waits for broker acknowledgement, which does not imply consumer processing.
// Cancellation stops confirmation waiting; delivery may already have occurred.
// The AMQP client cannot interrupt an in-flight socket write after it starts.
func (p *Publisher) Publish(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := MarshalEnvelope(envelope)
	if err != nil {
		return err
	}
	confirmed, err := p.ch.publish(ctx, eventExchange, envelope.GetType(), amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
	})
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	if confirmed == nil {
		return errors.New("publish event: broker confirmation unavailable")
	}
	acked, err := confirmed.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for broker confirmation: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !acked {
		return errors.New("publish event: broker did not acknowledge message")
	}
	return nil
}

// Close releases both owned resources and is safe to call more than once.
func (p *Publisher) Close() error {
	p.closeOnce.Do(func() {
		var channelErr error
		if p.ch != nil {
			channelErr = p.ch.Close()
			if errors.Is(channelErr, amqp.ErrClosed) {
				channelErr = nil
			}
		}
		connErr := p.conn.Close()
		if errors.Is(connErr, amqp.ErrClosed) {
			connErr = nil
		}
		p.closeErr = errors.Join(channelErr, connErr)
	})
	return p.closeErr
}

type rabbitConnection struct{ *amqp.Connection }

func (c rabbitConnection) channel() (publisherChannel, error) {
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}
	return rabbitChannel{ch}, nil
}

type rabbitChannel struct{ *amqp.Channel }

func (ch rabbitChannel) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) (confirmation, error) {
	dc, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, false, false, msg)
	if dc == nil {
		return nil, err
	}
	return dc, err
}
