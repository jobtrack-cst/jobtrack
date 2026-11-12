// Package forwarder delivers unchanged domain envelopes as HTTP CloudEvents.
package forwarder

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

const MaxAttempts = 4

// Acknowledger is the settlement subset needed by the transport.
type Acknowledger interface {
	Ack(bool) error
	Nack(bool, bool) error
	Reject(bool) error
}

// Handler performs one HTTP attempt per broker delivery. Retry state belongs to
// the quorum queue, never to a process or a publisher-supplied attempt header.
type Handler struct {
	Client *http.Client
	Sink   string
	Logger *slog.Logger
}

func NewHandler(sink string, logger *slog.Logger) *Handler {
	return &Handler{Sink: sink, Logger: logger, Client: &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// Handle requires a quorum delivery with the configured at-least-once DLX.
// An error means settlement is uncertain: the caller must close the session.
func (h *Handler) Handle(ctx context.Context, body []byte, headers amqp.Table, d Acknowledger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	envelope, err := events.UnmarshalEnvelope(body)
	if err != nil {
		return h.deadLetter(d, "", "malformed envelope")
	}
	count := int64(0)
	if value, ok := headers["x-delivery-count"]; ok {
		var valid bool
		count, valid = value.(int64)
		if !valid || count < 0 {
			return h.deadLetter(d, envelope.GetEventId(), "invalid delivery count")
		}
	}
	if count >= MaxAttempts {
		return h.deadLetter(d, envelope.GetEventId(), "delivery limit exhausted")
	}
	// Waiting before redelivery's HTTP attempt preserves the count if interrupted.
	if count > 0 {
		timer := time.NewTimer(time.Second << (count - 1))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Sink, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Ce-Specversion", "1.0")
	req.Header.Set("Ce-Id", envelope.GetEventId())
	req.Header.Set("Ce-Type", envelope.GetType())
	req.Header.Set("Ce-Source", "/jobtrack/rabbitmq")
	resp, err := h.Client.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			err := d.Ack(false)
			if err == nil && h.Logger != nil {
				h.Logger.Info("event delivered", "event_id", envelope.GetEventId())
			}
			return err
		}
	}
	if h.Logger != nil {
		h.Logger.Warn("HTTP delivery failed", "event_id", envelope.GetEventId(), "attempt", count+1)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if count == MaxAttempts-1 {
		return h.deadLetter(d, envelope.GetEventId(), "HTTP attempts exhausted")
	}
	// Reject (rather than basic.nack) counts a failed delivery on RabbitMQ 4.3 too.
	return d.Reject(true)
}

func (h *Handler) deadLetter(d Acknowledger, id, reason string) error {
	if h.Logger != nil {
		h.Logger.Warn("delivery dead-letter requested", "event_id", id, "reason", reason)
	}
	return d.Nack(false, false)
}

type Config struct {
	Broker, Sink, Function string
	Bindings               []string
}

var functionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,49}$`)
var bindingPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (c Config) Validate() error {
	u, err := url.Parse(c.Sink)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Fragment != "" || c.Broker == "" {
		return errors.New("RABBITMQ_URL and an HTTP SINK_URL are required")
	}
	return (Topology{Function: c.Function, Bindings: c.Bindings}).Validate()
}

// Dial cancels TCP establishment and AMQP handshake. Run manages cancellation
// of the returned connection, including blocked protocol operations.
func Dial(ctx context.Context, broker string) (*amqp.Connection, error) {
	var socket net.Conn
	var cancelWatch func() bool
	conn, err := amqp.DialConfig(broker, amqp.Config{Heartbeat: 10 * time.Second, Dial: func(network, addr string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 5 * time.Second}
		s, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		socket = s
		cancelWatch = context.AfterFunc(ctx, func() { _ = s.Close() })
		_ = s.SetDeadline(time.Now().Add(5 * time.Second))
		return s, nil
	}})
	if cancelWatch != nil {
		cancelWatch()
	}
	if err != nil {
		if socket != nil {
			_ = socket.Close()
		}
		return nil, errors.New("connect to RabbitMQ failed")
	}
	if ctx.Err() != nil {
		_ = conn.CloseDeadline(time.Now())
		return nil, ctx.Err()
	}
	return conn, nil
}

func Run(ctx context.Context, c Config, logger *slog.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}
	conn, err := Dial(ctx, c.Broker)
	if err != nil {
		return err
	}
	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { _ = conn.CloseDeadline(time.Now().Add(time.Second)) }) }
	stop := context.AfterFunc(ctx, closeConn)
	defer stop()
	defer closeConn()
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("open AMQP channel failed")
	}
	queue := c.Function + ".events"
	if err := (Topology{Function: c.Function, Bindings: c.Bindings}).Declare(ch); err != nil {
		return err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, queue, c.Function+"-forwarder", false, false, false, false, nil)
	if err != nil {
		return err
	}
	handler := NewHandler(c.Sink, logger)
	defer handler.Client.CloseIdleConnections()
	logger.Info("consumer ready", "queue", queue)
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-deliveries:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return errors.New("AMQP consumer closed")
			}
			if err := handler.Handle(ctx, msg.Body, msg.Headers, msg); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				id := ""
				if e, decodeErr := events.UnmarshalEnvelope(msg.Body); decodeErr == nil {
					id = e.GetEventId()
				}
				logger.Warn("delivery settlement failed; closing session", "event_id", id)
				return errors.New("delivery settlement failed")
			}
		}
	}
}
