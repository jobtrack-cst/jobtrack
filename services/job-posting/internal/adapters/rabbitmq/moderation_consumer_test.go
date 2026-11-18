package rabbitmq_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/rabbitmq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	fixtureEventID = "9b2e4c71-6d3a-4f85-8a1c-3e5f7a9b1c2d"
	fixtureJobID   = "33333333-3333-4333-8333-333333333333"
)

type removal struct{ eventID, jobID string }

type recorderStub struct {
	calls []removal
	err   error
}

func (s *recorderStub) RecordJobRemoved(_ context.Context, eventID, jobID string) error {
	s.calls = append(s.calls, removal{eventID, jobID})
	return s.err
}

func loadJobRemovedFixture(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..",
		"tests", "contract", "fixtures", "moderation.job_removed.json"))
	require.NoError(t, err)
	env, err := events.UnmarshalEnvelope(body)
	require.NoError(t, err)
	return env
}

func setData(t *testing.T, e *eventsv1.EventEnvelope, m proto.Message) {
	t.Helper()
	data, err := anypb.New(m)
	require.NoError(t, err)
	e.Data = data
}

func TestModerationHandler_Fixture_RecordsRemoval(t *testing.T) {
	rec := &recorderStub{}

	err := rabbitmq.ModerationHandler(rec)(context.Background(), loadJobRemovedFixture(t))

	require.NoError(t, err)
	assert.Equal(t, []removal{{fixtureEventID, fixtureJobID}}, rec.calls)
}

func TestModerationHandler_RejectsInvalidEvents(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *eventsv1.EventEnvelope)
	}{
		{"wrong producer", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.Producer = "job-posting" }},
		{"wrong type", func(_ *testing.T, e *eventsv1.EventEnvelope) { e.Type = "job.closed" }},
		{"wrong data type", func(t *testing.T, e *eventsv1.EventEnvelope) {
			setData(t, e, &eventsv1.JobClosed{JobId: fixtureJobID})
		}},
		{"job_id not a uuid", func(t *testing.T, e *eventsv1.EventEnvelope) {
			setData(t, e, &eventsv1.ModerationJobRemoved{JobId: "not-a-uuid", Reason: "x"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loadJobRemovedFixture(t)
			tt.mutate(t, env)
			rec := &recorderStub{}

			err := rabbitmq.ModerationHandler(rec)(context.Background(), env)

			require.Error(t, err)
			assert.Empty(t, rec.calls)
		})
	}
}

func TestModerationHandler_RepositoryError_IsReturnedForRetry(t *testing.T) {
	dbErr := errors.New("database unavailable")
	rec := &recorderStub{err: dbErr}

	err := rabbitmq.ModerationHandler(rec)(context.Background(), loadJobRemovedFixture(t))

	assert.ErrorIs(t, err, dbErr)
}
