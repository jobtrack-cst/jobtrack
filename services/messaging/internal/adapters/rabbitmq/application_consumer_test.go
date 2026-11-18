package rabbitmq_test

import (
	"context"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"testing"
)

type recorder struct {
	calls int
	err   error
}

func (r *recorder) RecordApplication(_ context.Context, _ string, p domain.ApplicationParticipants) error {
	r.calls++
	return r.err
}
func TestApplicationHandler_ValidatesSharedFixture(t *testing.T) {
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	for _, name := range []string{"valid", "producer", "type", "event ID", "payload", "timestamp", "bad participant", "same participants", "wrong payload"} {
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
			case "wrong payload":
				c.Data, _ = anypb.New(&eventsv1.MessageSent{})
			case "bad participant", "same participants":
				var d eventsv1.ApplicationSubmitted
				require.NoError(t, c.Data.UnmarshalTo(&d))
				if name == "bad participant" {
					d.JobId = "bad"
				} else {
					d.EmployerId = d.ApplicantId
				}
				c.Data, _ = anypb.New(&d)
			case "timestamp":
				c.OccurredAt = nil
			}
			err := rabbitmq.ApplicationHandler(r)(context.Background(), c)
			if name == "valid" {
				require.NoError(t, err)
				require.Equal(t, 1, r.calls)
			} else {
				require.Error(t, err)
				require.ErrorIs(t, err, events.ErrPermanent)
				require.Zero(t, r.calls)
			}
		})
	}
}

func TestApplicationHandler_RetriesRepositoryFailure(t *testing.T) {
	body, err := os.ReadFile("../../../../../tests/contract/fixtures/application.submitted.json")
	require.NoError(t, err)
	e, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	failure := errors.New("temporary database failure")
	r := &recorder{err: failure}
	err = rabbitmq.ApplicationHandler(r)(context.Background(), e)
	require.ErrorIs(t, err, failure)
	require.NotErrorIs(t, err, events.ErrPermanent)
}
