package rabbitmq_test

import (
	"context"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

type recorder struct{ calls int }

func (r *recorder) RecordApplication(_ context.Context, _ string, p domain.ApplicationParticipants) error {
	r.calls++
	return nil
}
func TestApplicationHandler_ValidatesSharedFixture(t *testing.T) {
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	for _, name := range []string{"valid", "producer", "type", "event ID", "payload", "timestamp"} {
		t.Run(name, func(t *testing.T) {
			c := proto.Clone(e).(*eventsv1.EventEnvelope)
			r := new(recorder)
			switch name {
			case "producer":
				c.Producer = "other"
			case "type":
				c.Type = "application.withdrawn"
			case "event ID":
				c.EventId = "bad"
			case "payload":
				c.Data = nil
			case "timestamp":
				c.OccurredAt = nil
			}
			err := rabbitmq.ApplicationHandler(r)(context.Background(), c)
			if name == "valid" {
				require.NoError(t, err)
				require.Equal(t, 1, r.calls)
			} else {
				require.Error(t, err)
				require.Zero(t, r.calls)
			}
		})
	}
}
