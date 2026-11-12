package rabbitmq

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

const ValidationQueue = "resume.validation"
const ValidationDeadQueue = "resume.validation.dead"
const validationDeliveries = 4

// DeclareValidationTopology only adds/validates this service's owned topology.
func DeclareValidationTopology(ch forwarder.TopologyChannel) error {
	if err := ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil); err != nil {
		return errors.New("declare validation exchange failed; check topology compatibility and permissions")
	}
	if _, err := ch.QueueDeclare(ValidationDeadQueue, true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
		return errors.New("declare validation dead queue failed; reviewed migration may be required")
	}
	args := amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int32(validationDeliveries - 1), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": ValidationDeadQueue, "x-dead-letter-strategy": "at-least-once", "x-overflow": "reject-publish"}
	if _, err := ch.QueueDeclare(ValidationQueue, true, false, false, false, args); err != nil {
		return errors.New("declare validation source queue failed; reviewed migration may be required")
	}
	for _, key := range []string{"resume.validated", "resume.rejected"} {
		if err := ch.QueueBind(ValidationQueue, key, "jobtrack.events", false, nil); err != nil {
			return errors.New("bind validation queue failed")
		}
	}
	return nil
}

// withValidationChannel owns a session; cancellation interrupts protocol I/O via
// bounded connection close, following the existing service topology pattern.
func withValidationChannel(ctx context.Context, broker string, run func(*amqp.Channel) error) error {
	conn, err := forwarder.Dial(ctx, broker)
	if err != nil {
		return errors.New("connect validation session failed")
	}
	var once sync.Once
	closeConn := func() { once.Do(func() { _ = conn.CloseDeadline(time.Now().Add(time.Second)) }) }
	stop := context.AfterFunc(ctx, closeConn)
	defer stop()
	defer closeConn()
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("open validation channel failed")
	}
	return run(ch)
}

// DecodeValidationDelivery validates transport identity before touching persistence.
func DecodeValidationDelivery(d amqp.Delivery) (domain.ValidationResult, error) {
	e, err := events.UnmarshalEnvelope(d.Body)
	if err != nil {
		return domain.ValidationResult{}, domain.ErrInvalidValidationResult
	}
	if d.Exchange != "jobtrack.events" || d.RoutingKey != e.Type || e.Producer != "resume-validator" {
		return domain.ValidationResult{}, domain.ErrInvalidValidationResult
	}
	switch e.Type {
	case "resume.validated":
		data := new(eventsv1.ResumeValidated)
		if e.Data.UnmarshalTo(data) != nil {
			return domain.ValidationResult{}, domain.ErrInvalidValidationResult
		}
		return domain.NewValidationResult(e.EventId, data.ResumeId, data.ApplicantId, domain.ValidationStatusValidated, "")
	case "resume.rejected":
		data := new(eventsv1.ResumeRejected)
		if e.Data.UnmarshalTo(data) != nil {
			return domain.ValidationResult{}, domain.ErrInvalidValidationResult
		}
		return domain.NewValidationResult(e.EventId, data.ResumeId, data.ApplicantId, domain.ValidationStatusRejected, data.Reason)
	default:
		return domain.ValidationResult{}, domain.ErrInvalidValidationResult
	}
}

type ValidationHandler struct {
	Results *app.ValidationResults
	Logger  *slog.Logger
}

// Handle never positively acknowledges before successful durable processing.
// A settlement error must close the session; caller must not attempt a second ack.
func (h *ValidationHandler) Handle(ctx context.Context, d amqp.Delivery) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	id := ""
	dead := func(reason string) error {
		if h.Logger != nil {
			h.Logger.Warn("validation dead-letter requested", "reason", reason, "event_id", id)
		}
		return d.Nack(false, false)
	}
	count := int64(0)
	if value, ok := d.Headers["x-delivery-count"]; ok {
		var valid bool
		count, valid = value.(int64)
		if !valid || count < 0 {
			return dead("invalid delivery count")
		}
	} else if d.Redelivered {
		return dead("missing redelivery count")
	}
	if count >= validationDeliveries {
		return dead("delivery budget exhausted")
	}
	result, err := DecodeValidationDelivery(d)
	if err != nil {
		return dead("invalid validation event")
	}
	id = result.EventID
	if count > 0 {
		if err := validationWait(ctx, time.Second<<uint(count-1)); err != nil {
			return err
		}
	}
	processing, cancel := context.WithTimeout(ctx, 2*time.Second)
	outcome, err := h.Results.Apply(processing, result)
	cancel()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		switch outcome {
		case domain.ValidationApplied, domain.ValidationDuplicate, domain.ValidationAlreadyFinal:
			return d.Ack(false)
		default:
			return errors.New("unexpected validation processing outcome")
		}
	}
	if errors.Is(err, domain.ErrValidationApplicantMismatch) || errors.Is(err, domain.ErrInvalidValidationResult) {
		return dead("invalid validation target or content")
	}
	if h.Logger != nil {
		h.Logger.Warn("validation processing failed", "event_id", result.EventID, "delivery", count+1)
	}
	if count == validationDeliveries-1 {
		return dead("processing retry budget exhausted")
	}
	// RabbitMQ counts basic.reject(requeue=true) as failed delivery across restarts.
	return d.Reject(true)
}

func validationWait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type ValidationConsumer struct {
	Broker  string
	Handler *ValidationHandler
	Logger  *slog.Logger
}

// Session processes one connection. Any uncertain settlement ends this session.
func (c *ValidationConsumer) Session(ctx context.Context) error {
	return withValidationChannel(ctx, c.Broker, func(ch *amqp.Channel) error {
		if err := DeclareValidationTopology(ch); err != nil {
			return err
		}
		if err := ch.Qos(1, 0, false); err != nil {
			return errors.New("configure validation prefetch failed")
		}
		deliveries, err := ch.ConsumeWithContext(ctx, ValidationQueue, "", false, false, false, false, nil)
		if err != nil {
			return errors.New("start validation consumption failed")
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case d, ok := <-deliveries:
				if !ok {
					return errors.New("validation deliveries closed")
				}
				if err := c.Handler.Handle(ctx, d); err != nil {
					return errors.New("validation processing or settlement interrupted")
				}
			}
		}
	})
}

func (c *ValidationConsumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		_ = c.Session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if c.Logger != nil {
			c.Logger.Warn("validation session ended; reconnecting")
		}
		if err := validationWait(ctx, 5*time.Second); err != nil {
			return nil
		}
	}
	return nil
}
