package forwarder_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

type topologyChannel struct {
	t      *testing.T
	failAt string
	calls  []string
}

func (c *topologyChannel) fail(stage string) error {
	c.calls = append(c.calls, stage)
	if c.failAt == stage {
		return errors.New("private-broker-detail")
	}
	return nil
}
func (c *topologyChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	require.Equal(c.t, "jobtrack.events", name)
	require.Equal(c.t, "topic", kind)
	require.True(c.t, durable)
	require.False(c.t, autoDelete || internal || noWait)
	require.Nil(c.t, args)
	return c.fail("exchange")
}
func (c *topologyChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	require.True(c.t, durable)
	require.False(c.t, autoDelete || exclusive || noWait)
	stage := "source"
	expected := amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int32(3), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": "resume-validator.events.dead", "x-dead-letter-strategy": "at-least-once", "x-overflow": "reject-publish"}
	if strings.HasSuffix(name, ".dead") {
		stage = "dead"
		require.Equal(c.t, "resume-validator.events.dead", name)
		expected = amqp.Table{"x-queue-type": "quorum"}
	} else {
		require.Equal(c.t, "resume-validator.events", name)
	}
	require.Equal(c.t, expected, args)
	return amqp.Queue{Name: name}, c.fail(stage)
}
func (c *topologyChannel) QueueBind(queue, key, exchange string, noWait bool, args amqp.Table) error {
	require.Equal(c.t, "resume-validator.events", queue)
	require.Equal(c.t, "resume.uploaded", key)
	require.Equal(c.t, "jobtrack.events", exchange)
	require.False(c.t, noWait)
	require.Nil(c.t, args)
	return c.fail("bind")
}

func TestTopology_UsesExactJT61ArgumentsAndFailsClosed(t *testing.T) {
	for _, stage := range []string{"", "exchange", "dead", "source", "bind"} {
		t.Run(stage, func(t *testing.T) {
			ch := &topologyChannel{t: t, failAt: stage}
			err := forwarder.ResumeValidatorTopology().Declare(ch)
			want := []string{"exchange", "dead", "source", "bind"}
			if stage != "" {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-broker-detail")
				for i, s := range want {
					if s == stage {
						want = want[:i+1]
						break
					}
				}
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, want, ch.calls)
		})
	}
}
func TestTopology_RejectsInvalidConfigBeforeDeclarations(t *testing.T) {
	for _, topology := range []forwarder.Topology{{}, {Function: "../bad", Bindings: []string{"resume.uploaded"}}, {Function: "resume-validator", Bindings: []string{"#"}}} {
		ch := &topologyChannel{t: t}
		require.Error(t, topology.Declare(ch))
		require.Empty(t, ch.calls)
	}
}
