package rabbitmq

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ProvisionTopology establishes the validator inbox even when functions are off.
// The JT-61 helper is the single definition of queue arguments and bindings.
func ProvisionTopology(ctx context.Context, broker string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := forwarder.EnsureTopology(ctx, broker, forwarder.ResumeValidatorTopology()); err != nil {
		return err
	}
	return withValidationChannel(ctx, broker, func(ch *amqp.Channel) error { return DeclareValidationTopology(ch) })
}
