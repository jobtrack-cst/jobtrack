package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MaxDeliveries matches the JT-61 policy: at most four attempts per event,
// then the broker moves it to the <queue>.dead queue.
const MaxDeliveries = 4

// HandlerTimeout bounds one handler call, so a stuck database call cannot stop
// the queue; a timed-out event is retried like any other temporary failure.
const HandlerTimeout = 10 * time.Second

// ErrPermanent marks a handler error that retrying cannot fix, such as an
// event with an invalid ID. The event goes straight to the dead queue.
var ErrPermanent = errors.New("permanent event failure")

// Permanent wraps err so the consumer dead-letters the event without retrying.
func Permanent(err error) error {
	return fmt.Errorf("%w: %w", ErrPermanent, err)
}

// Handler processes one decoded event. Return nil to acknowledge it, an error
// wrapped with Permanent to dead-letter it, or any other error to retry it.
// Handlers must be idempotent: the same event_id can arrive more than once.
type Handler func(ctx context.Context, envelope *eventsv1.EventEnvelope) error

// ConsumerConfig describes one service queue, named <service>.<purpose>
// (AGENTS.md Section 9).
type ConsumerConfig struct {
	URL      string
	Queue    string
	Bindings []string
	Handler  Handler
	Logger   *slog.Logger
	// OnReady, if set, is called each time the queue is declared, bound and
	// consuming. Tests use it to know when they can publish.
	OnReady func()

	// Unexported so production always uses the JT-61 timings; tests in this
	// package shorten them.
	retryBase      time.Duration
	handlerTimeout time.Duration
	reconnectBase  time.Duration
	reconnectMax   time.Duration
}

var (
	queueNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*\.[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	routingKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
)

func (c *ConsumerConfig) validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	if !queueNamePattern.MatchString(c.Queue) {
		return errors.New("queue must be named <service>.<purpose> in kebab-case")
	}
	if len(c.Bindings) == 0 {
		return errors.New("at least one routing key binding is required")
	}
	for _, b := range c.Bindings {
		if !routingKeyPattern.MatchString(b) {
			return fmt.Errorf("binding %q is not an event routing key", b)
		}
	}
	if c.Handler == nil {
		return errors.New("handler is required")
	}
	if c.Logger == nil {
		return errors.New("logger is required")
	}
	if c.retryBase <= 0 {
		c.retryBase = time.Second
	}
	if c.handlerTimeout <= 0 {
		c.handlerTimeout = HandlerTimeout
	}
	if c.reconnectBase <= 0 {
		c.reconnectBase = time.Second
	}
	if c.reconnectMax <= 0 {
		c.reconnectMax = 30 * time.Second
	}
	return nil
}

// sourceQueueArgs are the JT-61 quorum queue settings, so the dead queue keeps
// any event that fails MaxDeliveries times.
func sourceQueueArgs(queue string) amqp.Table {
	return amqp.Table{
		"x-queue-type":              "quorum",
		"x-delivery-limit":          int32(MaxDeliveries - 1),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": queue + ".dead",
		"x-dead-letter-strategy":    "at-least-once",
		"x-overflow":                "reject-publish",
	}
}

// Consume runs until ctx is cancelled and then returns nil. If the broker
// connection drops it reconnects with backoff, so a RabbitMQ restart does not
// stop the service. It returns an error only for an invalid configuration.
func Consume(ctx context.Context, cfg ConsumerConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	wait := cfg.reconnectBase
	for {
		ready, err := consumeSession(ctx, &cfg)
		if ctx.Err() != nil {
			return nil
		}
		if ready {
			wait = cfg.reconnectBase
		}
		cfg.Logger.Warn("event consumer stopped; reconnecting", "queue", cfg.Queue, "error", err, "retry_in", wait.String())
		if !sleep(ctx, wait) {
			return nil
		}
		wait = min(wait*2, cfg.reconnectMax)
	}
}

// consumeSession declares the topology and handles deliveries until the
// connection fails or ctx ends. ready reports whether consuming started.
func consumeSession(ctx context.Context, cfg *ConsumerConfig) (ready bool, err error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		// Dial errors may contain the URL, including credentials.
		return false, errors.New("connect to RabbitMQ failed; check RABBITMQ_URL and broker availability")
	}
	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { _ = conn.CloseDeadline(time.Now().Add(time.Second)) }) }
	stop := context.AfterFunc(ctx, closeConn)
	defer stop()
	defer closeConn()

	ch, err := conn.Channel()
	if err != nil {
		return false, errors.New("open AMQP channel failed")
	}
	if err := ch.ExchangeDeclare(eventExchange, "topic", true, false, false, false, nil); err != nil {
		return false, fmt.Errorf("declare event exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(cfg.Queue+".dead", true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
		return false, fmt.Errorf("declare dead queue (existing classic queues require migration): %w", err)
	}
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, sourceQueueArgs(cfg.Queue)); err != nil {
		return false, fmt.Errorf("declare source queue (existing classic queues require migration): %w", err)
	}
	for _, key := range cfg.Bindings {
		if err := ch.QueueBind(cfg.Queue, key, eventExchange, false, nil); err != nil {
			return false, fmt.Errorf("bind %s: %w", key, err)
		}
	}
	// One unacknowledged event at a time keeps processing in queue order.
	if err := ch.Qos(1, 0, false); err != nil {
		return false, fmt.Errorf("set prefetch: %w", err)
	}
	deliveries, err := ch.ConsumeWithContext(ctx, cfg.Queue, cfg.Queue+"-consumer", false, false, false, false, nil)
	if err != nil {
		return false, fmt.Errorf("start consuming: %w", err)
	}
	cfg.Logger.Info("event consumer ready", "queue", cfg.Queue)
	if cfg.OnReady != nil {
		cfg.OnReady()
	}
	for {
		select {
		case <-ctx.Done():
			return true, nil
		case msg, ok := <-deliveries:
			if !ok {
				return true, errors.New("AMQP delivery channel closed")
			}
			if err := settle(ctx, cfg, msg.Body, msg.Headers, msg); err != nil {
				if ctx.Err() != nil {
					return true, nil
				}
				return true, fmt.Errorf("settle delivery: %w", err)
			}
		}
	}
}

// acknowledger is the subset of amqp.Delivery the consumer uses.
type acknowledger interface {
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
	Reject(requeue bool) error
}

// settle runs the handler for one delivery and acknowledges, retries or
// dead-letters it. The message body is never logged.
func settle(ctx context.Context, cfg *ConsumerConfig, body []byte, headers amqp.Table, d acknowledger) error {
	envelope, err := UnmarshalEnvelope(body)
	if err != nil {
		cfg.Logger.Warn("malformed event dead-lettered", "queue", cfg.Queue)
		return d.Nack(false, false)
	}
	log := cfg.Logger.With("queue", cfg.Queue, "event_id", envelope.GetEventId(), "event_type", envelope.GetType())

	count, ok := deliveryCount(headers)
	if !ok {
		log.Warn("event with invalid delivery count dead-lettered")
		return d.Nack(false, false)
	}
	// Waiting before a redelivered attempt keeps the count if interrupted.
	if count > 0 && !sleep(ctx, cfg.retryBase<<(count-1)) {
		return d.Reject(true)
	}

	handlerCtx, cancel := context.WithTimeout(ctx, cfg.handlerTimeout)
	err = cfg.Handler(handlerCtx, envelope)
	cancel()
	switch {
	case err == nil:
		return d.Ack(false)
	case errors.Is(err, ErrPermanent):
		log.Warn("event rejected by handler; dead-lettered", "error", err)
		return d.Nack(false, false)
	case ctx.Err() != nil:
		// Shutting down: give the event back without counting it as handled.
		return d.Reject(true)
	case count >= MaxDeliveries-1:
		log.Error("event failed on every attempt; dead-lettered", "error", err, "attempt", count+1)
		return d.Nack(false, false)
	default:
		log.Warn("event handling failed; will retry", "error", err, "attempt", count+1)
		// Reject, not basic.nack, so RabbitMQ counts the failed delivery.
		return d.Reject(true)
	}
}

// deliveryCount reads the quorum queue's x-delivery-count header, which is
// absent on the first delivery.
func deliveryCount(headers amqp.Table) (int64, bool) {
	value, ok := headers["x-delivery-count"]
	if !ok {
		return 0, true
	}
	count, ok := value.(int64)
	if !ok || count < 0 {
		return 0, false
	}
	return count, true
}

// sleep waits for d or until ctx ends, and reports whether the full wait passed.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
