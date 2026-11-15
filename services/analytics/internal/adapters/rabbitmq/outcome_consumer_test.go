package rabbitmq_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/anypb"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/ports/mocks"
)

// loadFixture reads a shared fixture from tests/contract, so producers and
// analytics test against the same JSON.
func loadFixture(t *testing.T, routingKey string) *eventsv1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", routingKey+".json"))
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	return envelope
}

func newHandler(t *testing.T) (events.Handler, *mocks.MockOutcomeRepository, *bytes.Buffer) {
	repo := mocks.NewMockOutcomeRepository(gomock.NewController(t))
	logs := &bytes.Buffer{}
	return rabbitmq.OutcomeHandler(app.NewAnalytics(repo), slog.New(slog.NewJSONHandler(logs, nil))), repo, logs
}

func TestOutcomeHandler_RecordsAllEightSharedFixtures(t *testing.T) {
	for _, tt := range []struct{ key, status string }{
		{"job.published", ""}, {"job.closed", ""},
		{"application.submitted", ""}, {"application.withdrawn", ""},
		{"application.status_changed", "UNDER_REVIEW"},
		{"offer.made", ""}, {"offer.accepted", ""}, {"offer.declined", ""},
	} {
		t.Run(tt.key, func(t *testing.T) {
			handler, repo, logs := newHandler(t)
			envelope := loadFixture(t, tt.key)
			want := domain.OutcomeEvent{
				EventID: envelope.GetEventId(), EventType: tt.key,
				NewStatus: tt.status, OccurredAt: envelope.GetOccurredAt().AsTime(),
			}
			repo.EXPECT().RecordEvent(gomock.Any(), want).Return(true, nil).Times(1)
			require.NoError(t, handler(context.Background(), envelope))
			require.Contains(t, logs.String(), `"recorded":true`)
			require.Contains(t, logs.String(), want.EventID)
		})
	}
}

func TestOutcomeHandler_DuplicateIsAcknowledged(t *testing.T) {
	handler, repo, logs := newHandler(t)
	repo.EXPECT().RecordEvent(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
	require.NoError(t, handler(context.Background(), loadFixture(t, "offer.made")))
	require.Contains(t, logs.String(), `"recorded":false`)
}

func TestOutcomeHandler_DatabaseErrorIsRetried(t *testing.T) {
	handler, repo, _ := newHandler(t)
	errDB := errors.New("database down")
	repo.EXPECT().RecordEvent(gomock.Any(), gomock.Any()).Return(false, errDB)
	err := handler(context.Background(), loadFixture(t, "job.published"))
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, events.ErrPermanent, "a temporary failure must be retried, not dead-lettered")
}

func TestOutcomeHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	withData := func(m *eventsv1.StatusChanged) func(*eventsv1.EventEnvelope) {
		return func(e *eventsv1.EventEnvelope) {
			data, err := anypb.New(m)
			require.NoError(t, err)
			e.Data = data
		}
	}
	jobClosed, err := anypb.New(&eventsv1.JobClosed{JobId: "33333333-3333-4333-8333-333333333333"})
	require.NoError(t, err)
	for _, tt := range []struct {
		name    string
		fixture string
		change  func(*eventsv1.EventEnvelope)
	}{
		{"type not counted", "job.published", func(e *eventsv1.EventEnvelope) { e.Type = "job.updated" }},
		{"job event not from job-posting", "job.published", func(e *eventsv1.EventEnvelope) { e.Producer = "application" }},
		{"application event not from application", "application.submitted", func(e *eventsv1.EventEnvelope) { e.Producer = "job-posting" }},
		{"status change not from application-status", "application.status_changed", func(e *eventsv1.EventEnvelope) { e.Producer = "application" }},
		{"offer event not from offer", "offer.accepted", func(e *eventsv1.EventEnvelope) { e.Producer = "application" }},
		{"data does not match the routing key", "application.submitted", func(e *eventsv1.EventEnvelope) { e.Data = jobClosed }},
		{"missing data", "offer.made", func(e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"missing occurred_at", "job.closed", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"event ID not a UUID", "offer.declined", func(e *eventsv1.EventEnvelope) { e.EventId = "event-1" }},
		{"status change without new_status", "application.status_changed", withData(&eventsv1.StatusChanged{PreviousStatus: "SUBMITTED"})},
		{"new_status of 33 characters", "application.status_changed", withData(&eventsv1.StatusChanged{NewStatus: strings.Repeat("S", 33)})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _ := newHandler(t) // the mock fails the test if the repository is called
			envelope := loadFixture(t, tt.fixture)
			tt.change(envelope)
			err := handler(context.Background(), envelope)
			require.ErrorIs(t, err, events.ErrPermanent)
			require.ErrorIs(t, err, domain.ErrInvalidOutcomeEvent)
		})
	}
}

func TestOutcomeConsumerConfig_UsesOutcomesQueueAndEightEvents(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	cfg := rabbitmq.OutcomeConsumerConfig("amqp://broker", logger, nil)
	require.Equal(t, "analytics.outcomes", cfg.Queue)
	require.ElementsMatch(t, []string{
		"job.published", "job.closed", "application.submitted", "application.withdrawn",
		"application.status_changed", "offer.made", "offer.accepted", "offer.declined",
	}, cfg.Bindings)
	require.Equal(t, "amqp://broker", cfg.URL)
	require.NotNil(t, cfg.Handler)
	require.Same(t, logger, cfg.Logger)
}
