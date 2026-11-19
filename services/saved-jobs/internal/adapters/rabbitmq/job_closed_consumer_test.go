package rabbitmq_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/ports/mocks"
)

// Values in tests/contract/fixtures/job.closed.json, shared with job-posting.
const (
	fixtureJobClosedEventID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2"
	fixtureClosedJobID      = "33333333-3333-4333-8333-333333333333"
)

var fixtureClosedAt = time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

func loadJobClosedFixture(t *testing.T) *eventsv1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", "job.closed.json"))
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	return envelope
}

func newJobClosedHandler(t *testing.T) (events.Handler, *mocks.MockRepository, *bytes.Buffer) {
	t.Helper()
	repo := mocks.NewMockRepository(gomock.NewController(t))
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	return rabbitmq.JobClosedHandler(app.NewSavedJobs(repo), logger), repo, logs
}

func TestJobClosedHandler_AppliesSharedFixture(t *testing.T) {
	handler, repo, logs := newJobClosedHandler(t)
	want := domain.JobClosure{EventID: fixtureJobClosedEventID, JobID: fixtureClosedJobID, ClosedAt: fixtureClosedAt}
	repo.EXPECT().CloseJob(gomock.Any(), want).Return(domain.ClosureOutcome{RemovedBookmarks: 2}, nil)

	require.NoError(t, handler(context.Background(), loadJobClosedFixture(t)))
	require.Contains(t, logs.String(), fixtureJobClosedEventID)
	require.Contains(t, logs.String(), `"removed_bookmarks":2`)
}

func TestJobClosedHandler_DuplicateIsAcknowledged(t *testing.T) {
	handler, repo, logs := newJobClosedHandler(t)
	repo.EXPECT().CloseJob(gomock.Any(), gomock.Any()).Return(domain.ClosureOutcome{Duplicate: true}, nil)

	require.NoError(t, handler(context.Background(), loadJobClosedFixture(t)))
	require.Contains(t, logs.String(), `"duplicate":true`)
}

func TestJobClosedHandler_DatabaseErrorIsRetried(t *testing.T) {
	handler, repo, _ := newJobClosedHandler(t)
	errDB := errors.New("database down")
	repo.EXPECT().CloseJob(gomock.Any(), gomock.Any()).Return(domain.ClosureOutcome{}, errDB)

	err := handler(context.Background(), loadJobClosedFixture(t))
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, events.ErrPermanent, "a temporary failure must be retried, not dead-lettered")
}

func TestJobClosedHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	otherData, err := anypb.New(&eventsv1.EmployerVerified{EmployerId: fixtureClosedJobID})
	require.NoError(t, err)
	jobClosedData := func(jobID string) *anypb.Any {
		data, err := anypb.New(&eventsv1.JobClosed{JobId: jobID})
		require.NoError(t, err)
		return data
	}
	tests := []struct {
		name   string
		change func(e *eventsv1.EventEnvelope)
	}{
		{"other routing key", func(e *eventsv1.EventEnvelope) { e.Type = "job.published" }},
		{"producer is not job-posting", func(e *eventsv1.EventEnvelope) { e.Producer = "moderation" }},
		{"data does not match the routing key", func(e *eventsv1.EventEnvelope) { e.Data = otherData }},
		{"missing data", func(e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"missing occurred_at", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"invalid occurred_at", func(e *eventsv1.EventEnvelope) { e.OccurredAt = &timestamppb.Timestamp{Seconds: 1, Nanos: -1} }},
		{"event ID not a UUID", func(e *eventsv1.EventEnvelope) { e.EventId = "event-1" }},
		{"job ID not a UUID", func(e *eventsv1.EventEnvelope) { e.Data = jobClosedData("job-1") }},
		{"empty job ID", func(e *eventsv1.EventEnvelope) { e.Data = jobClosedData("") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _ := newJobClosedHandler(t) // the mock fails the test if the repository is called
			envelope := loadJobClosedFixture(t)
			tt.change(envelope)
			err := handler(context.Background(), envelope)
			require.ErrorIs(t, err, events.ErrPermanent)
			require.ErrorIs(t, err, domain.ErrInvalidJobClosedEvent)
		})
	}
}

func TestJobClosedConsumerConfig_UsesSavedJobsQueue(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	cfg := rabbitmq.JobClosedConsumerConfig("amqp://broker", logger, nil)
	require.Equal(t, "saved-jobs.jobs", cfg.Queue)
	require.Equal(t, []string{"job.closed"}, cfg.Bindings)
	require.Equal(t, "amqp://broker", cfg.URL)
	require.NotNil(t, cfg.Handler)
	require.Same(t, logger, cfg.Logger)
}
