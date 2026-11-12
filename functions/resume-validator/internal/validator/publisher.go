package validator

import (
	"context"
	"errors"
	"sync"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	amqp "github.com/rabbitmq/amqp091-go"
)

// BrokerPublisher uses the shared envelope codec and approved AMQP client. The
// shared Publisher has no cancellable construction/close API; keep this bounded
// adapter local rather than changing other services' shared API in JT-62.
type BrokerPublisher struct{ URL string }

func (p BrokerPublisher) Publish(ctx context.Context, e *eventsv1.EventEnvelope) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := events.MarshalEnvelope(e)
	if err != nil {
		return err
	}
	conn, err := forwarder.Dial(ctx, p.URL)
	if err != nil {
		return errors.New("connect validation publisher failed")
	}
	var once sync.Once
	closeConn := func() { once.Do(func() { _ = conn.CloseDeadline(time.Now().Add(100 * time.Millisecond)) }) }
	// Closing the socket bounds channel creation, declare/confirm RPCs and socket
	// writes too, not just the deferred confirmation wait.
	stop := context.AfterFunc(ctx, closeConn)
	defer stop()
	defer closeConn()
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("open validation publisher channel failed")
	}
	if err := ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil); err != nil {
		return errors.New("declare validation exchange failed")
	}
	if err := ch.Confirm(false); err != nil {
		return errors.New("enable validation confirmations failed")
	}
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, "jobtrack.events", e.Type, false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body})
	if err != nil {
		return errors.New("publish validation failed")
	}
	if confirmation == nil {
		return errors.New("validation confirmation unavailable")
	}
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return errors.New("wait for validation confirmation failed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !acked {
		return errors.New("broker rejected validation publication")
	}
	return nil
}
