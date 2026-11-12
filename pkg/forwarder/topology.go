package forwarder

import (
	"context"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Topology is the JT-61 durable function inbox and dead-letter destination.
// Provisioning declares and binds only; it never consumes or publishes messages.
type Topology struct {
	Function string
	Bindings []string
}

func ResumeValidatorTopology() Topology {
	return Topology{Function: "resume-validator", Bindings: []string{"resume.uploaded"}}
}

func (t Topology) Validate() error {
	if !functionPattern.MatchString(t.Function) || len(t.Bindings) == 0 {
		return errors.New("FUNCTION_NAME and EVENT_TYPES are required")
	}
	for _, b := range t.Bindings {
		if !bindingPattern.MatchString(b) {
			return errors.New("EVENT_TYPES must contain exact event types")
		}
	}
	return nil
}

// TopologyChannel permits testing declarations without changing broker state.
type TopologyChannel interface {
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	QueueBind(string, string, string, bool, amqp.Table) error
}

// Declare is shared by the forwarder and independent startup provisioning.
// Equivalent declarations retain messages. Mismatches fail without migration.
func (t Topology) Declare(ch TopologyChannel) error {
	if err := t.Validate(); err != nil {
		return err
	}
	queue := t.Function + ".events"
	if err := ch.ExchangeDeclare("jobtrack.events", "topic", true, false, false, false, nil); err != nil {
		return errors.New("declare jobtrack.events failed; verify broker access and topology compatibility")
	}
	if _, err := ch.QueueDeclare(queue+".dead", true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
		return errors.New("declare function dead queue failed; verify broker access or arrange reviewed topology migration")
	}
	args := amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int32(MaxAttempts - 1),
		"x-dead-letter-exchange": "", "x-dead-letter-routing-key": queue + ".dead",
		"x-dead-letter-strategy": "at-least-once", "x-overflow": "reject-publish"}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, args); err != nil {
		return errors.New("declare function source queue failed; verify broker access or arrange reviewed topology migration")
	}
	for _, binding := range t.Bindings {
		if err := ch.QueueBind(queue, binding, "jobtrack.events", false, nil); err != nil {
			return errors.New("bind function source queue failed; verify broker access and topology compatibility")
		}
	}
	return nil
}

// EnsureTopology returns only after synchronous broker declarations and bindings.
// The caller controls the setup deadline. No consumer or HTTP sink is needed.
func EnsureTopology(ctx context.Context, broker string, topology Topology) error {
	if err := topology.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := Dial(ctx, broker)
	if err != nil {
		return errors.New("connect for topology provisioning failed")
	}
	var once sync.Once
	closeConn := func() { once.Do(func() { _ = conn.CloseDeadline(time.Now().Add(time.Second)) }) }
	stop := context.AfterFunc(ctx, closeConn)
	defer stop()
	defer closeConn()
	ch, err := conn.Channel()
	if err != nil {
		return errors.New("open topology provisioning channel failed")
	}
	if err := topology.Declare(ch); err != nil {
		return err
	}
	return ctx.Err()
}
