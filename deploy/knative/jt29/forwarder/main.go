// The JT-29 transport spike is not a production microservice or serverless function.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

type delivery interface {
	Ack(bool) error
	Nack(bool, bool) error
}

func forward(ctx context.Context, client *http.Client, sink string, body []byte, d delivery) error {
	envelope, err := events.UnmarshalEnvelope(body)
	if err != nil {
		// Reject poison messages. Production needs a dead-letter policy.
		_ = d.Nack(false, false)
		return errors.New("invalid event envelope rejected")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sink, bytes.NewReader(body))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Ce-Specversion", "1.0")
		req.Header.Set("Ce-Id", envelope.GetEventId())
		req.Header.Set("Ce-Type", envelope.GetType())
		req.Header.Set("Ce-Source", "/jobtrack/jt29/rabbitmq")
		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return d.Ack(false)
			}
		}
	}
	_ = d.Nack(false, true)
	return errors.New("HTTP delivery failed; event requeued")
}

func run(ctx context.Context, broker, sink string) error {
	u, err := url.Parse(sink)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || broker == "" {
		return errors.New("RABBITMQ_URL and an HTTP SINK_URL are required")
	}
	conn, err := amqp.Dial(broker)
	if err != nil {
		return errors.New("connect to RabbitMQ failed")
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("open AMQP channel failed")
	}
	defer ch.Close()
	if err := ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil); err != nil {
		return errors.New("declare exchange failed")
	}
	q, err := ch.QueueDeclare("jt29.hello", true, false, false, false, nil)
	if err != nil {
		return errors.New("declare spike queue failed")
	}
	if err := ch.QueueBind(q.Name, "#", "jobtrack.events", false, nil); err != nil {
		return errors.New("bind spike queue failed")
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return errors.New("set prefetch failed")
	}
	msgs, err := ch.ConsumeWithContext(ctx, q.Name, "jt29-forwarder", false, false, false, false, nil)
	if err != nil {
		return errors.New("start consumer failed")
	}
	logger := logging.New("jt29-forwarder", os.Stdout)
	logger.Info("consumer ready")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return errors.New("AMQP consumer closed")
			}
			if err := forward(ctx, client, sink, msg.Body, msg); err != nil {
				logger.Warn("delivery unsuccessful", "error", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(time.Second):
				}
			} else {
				envelope, _ := events.UnmarshalEnvelope(msg.Body)
				logger.Info("event delivered", "event_id", envelope.GetEventId())
			}
		}
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv("RABBITMQ_URL"), os.Getenv("SINK_URL")); err != nil {
		logging.New("jt29-forwarder", os.Stdout).Error("forwarder stopped", "error", err)
		os.Exit(1)
	}
}
