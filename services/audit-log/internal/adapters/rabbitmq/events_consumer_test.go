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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports/mocks"
)

// The shared fixtures in tests/contract/fixtures, one per routing key.
var fixturesDir = filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures")

func loadFixture(t *testing.T, name string) *eventsv1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixturesDir, name))
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	return envelope
}

func newEventsHandler(t *testing.T) (events.Handler, *mocks.MockEntryRepository, *bytes.Buffer) {
	t.Helper()
	repo := mocks.NewMockEntryRepository(gomock.NewController(t))
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	return rabbitmq.EventsHandler(app.NewAuditLog(repo), logger), repo, logs
}

func TestEventsHandler_RecordsEverySharedFixture(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(fixturesDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no shared fixtures found")
	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			handler, repo, logs := newEventsHandler(t)
			envelope := loadFixture(t, name)
			repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e domain.Entry) (bool, error) {
				require.Equal(t, strings.ToLower(envelope.GetEventId()), e.EventID)
				require.Equal(t, envelope.GetType(), e.EventType)
				require.Equal(t, envelope.GetProducer(), e.Producer)
				require.True(t, envelope.GetOccurredAt().AsTime().Equal(e.OccurredAt))
				want, err := protojson.Marshal(envelope.GetData())
				require.NoError(t, err)
				require.JSONEq(t, string(want), e.DataJSON)
				require.Contains(t, e.DataJSON, "@type")
				return true, nil
			})

			require.NoError(t, handler(context.Background(), envelope))
			require.Contains(t, logs.String(), `"duplicate":false`)
		})
	}
}

func TestEventsHandler_DuplicateIsAcknowledged(t *testing.T) {
	handler, repo, logs := newEventsHandler(t)
	repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).Return(false, nil)

	require.NoError(t, handler(context.Background(), loadFixture(t, "job.closed.json")))
	require.Contains(t, logs.String(), `"duplicate":true`)
}

func TestEventsHandler_DatabaseErrorIsRetried(t *testing.T) {
	handler, repo, _ := newEventsHandler(t)
	errDB := errors.New("database down")
	repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).Return(false, errDB)

	err := handler(context.Background(), loadFixture(t, "job.closed.json"))
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, events.ErrPermanent, "a temporary failure must be retried, not dead-lettered")
}

func TestEventsHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	oversized, err := anypb.New(wrapperspb.String(strings.Repeat("a", domain.MaxDataJSONBytes)))
	require.NoError(t, err)
	tests := []struct {
		name   string
		change func(e *eventsv1.EventEnvelope)
	}{
		{"event ID not a UUID", func(e *eventsv1.EventEnvelope) { e.EventId = "event-1" }},
		{"type not a routing key", func(e *eventsv1.EventEnvelope) { e.Type = "JobClosed" }},
		{"producer not kebab-case", func(e *eventsv1.EventEnvelope) { e.Producer = "Job_Posting" }},
		{"missing occurred_at", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"invalid occurred_at", func(e *eventsv1.EventEnvelope) { e.OccurredAt = &timestamppb.Timestamp{Seconds: 1, Nanos: -1} }},
		{"missing data", func(e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"data over 65536 bytes", func(e *eventsv1.EventEnvelope) { e.Data = oversized }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, _ := newEventsHandler(t) // the mock fails the test if the repository is called
			envelope := loadFixture(t, "job.closed.json")
			tt.change(envelope)
			err := handler(context.Background(), envelope)
			require.ErrorIs(t, err, events.ErrPermanent)
			require.ErrorIs(t, err, app.ErrInvalidEntry)
		})
	}
}

func TestEventsConsumerConfig_UsesAuditLogQueueForAllEvents(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	cfg := rabbitmq.EventsConsumerConfig("amqp://broker", logger, nil)
	require.Equal(t, "audit-log.events", cfg.Queue)
	require.Equal(t, []string{"#"}, cfg.Bindings)
	require.Equal(t, "amqp://broker", cfg.URL)
	require.NotNil(t, cfg.Handler)
	require.Same(t, logger, cfg.Logger)
}
