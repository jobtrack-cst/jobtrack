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

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports/mocks"
)

// The shared fixtures from tests/contract, so producer and consumer agree.
const fixtureEmployerID = "55555555-5555-4555-8555-555555555555"

func loadFixture(t *testing.T, name string) *eventsv1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tests", "contract", "fixtures", name))
	require.NoError(t, err)
	envelope, err := events.UnmarshalEnvelope(raw)
	require.NoError(t, err)
	return envelope
}

var handlerNow = time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)

type handlerDeps struct {
	repo *mocks.MockVerificationRepository
	logs *bytes.Buffer
}

func newHandler(t *testing.T) (events.Handler, handlerDeps) {
	ctrl := gomock.NewController(t)
	d := handlerDeps{repo: mocks.NewMockVerificationRepository(ctrl), logs: &bytes.Buffer{}}
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(handlerNow).AnyTimes()
	logger := slog.New(slog.NewJSONHandler(d.logs, nil))
	return rabbitmq.VerificationHandler(app.NewVerifications(d.repo, clock), logger), d
}

func TestVerificationHandler_AppliesSharedFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		want    domain.VerificationDecision
	}{
		{"employer.verified.json", domain.VerificationDecision{
			EventID: "8a1f0c52-3d4e-4b6a-9c7d-1e2f3a4b5c6d", EmployerID: fixtureEmployerID,
			Status: domain.VerificationVerified, DecidedAt: time.Date(2026, 11, 12, 4, 30, 0, 123456000, time.UTC),
		}},
		{"employer.verification_rejected.json", domain.VerificationDecision{
			EventID: "9b2e1d63-4e5f-4c7b-8d8e-2f3a4b5c6d7e", EmployerID: fixtureEmployerID,
			Status: domain.VerificationRejected, RejectionReason: "Registration number does not match the trade licence.",
			DecidedAt: time.Date(2026, 11, 12, 4, 0, 0, 654321000, time.UTC),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			handler, d := newHandler(t)
			d.repo.EXPECT().ApplyVerification(gomock.Any(), tt.want, handlerNow).Return(domain.OutcomeApplied, nil)
			require.NoError(t, handler(context.Background(), loadFixture(t, tt.fixture)))
			require.Contains(t, d.logs.String(), `"outcome":"applied"`)
			require.Contains(t, d.logs.String(), tt.want.EventID)
		})
	}
}

func TestVerificationHandler_DuplicateAndStaleAreAcknowledged(t *testing.T) {
	for _, outcome := range []domain.VerificationOutcome{domain.OutcomeDuplicate, domain.OutcomeStale, domain.OutcomeStoredWithoutProfile} {
		t.Run(outcome.String(), func(t *testing.T) {
			handler, d := newHandler(t)
			d.repo.EXPECT().ApplyVerification(gomock.Any(), gomock.Any(), gomock.Any()).Return(outcome, nil)
			require.NoError(t, handler(context.Background(), loadFixture(t, "employer.verified.json")))
			require.Contains(t, d.logs.String(), outcome.String())
		})
	}
}

func TestVerificationHandler_DatabaseErrorIsRetried(t *testing.T) {
	handler, d := newHandler(t)
	errDB := errors.New("database down")
	d.repo.EXPECT().ApplyVerification(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.VerificationOutcome(0), errDB)
	err := handler(context.Background(), loadFixture(t, "employer.verified.json"))
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, events.ErrPermanent, "a temporary failure must be retried, not dead-lettered")
}

func TestVerificationHandler_InvalidEventsAreDeadLettered(t *testing.T) {
	rejectedData, err := anypb.New(&eventsv1.EmployerVerificationRejected{EmployerId: fixtureEmployerID, Reason: "x"})
	require.NoError(t, err)
	tests := []struct {
		name   string
		change func(e *eventsv1.EventEnvelope)
	}{
		{"unknown routing key", func(e *eventsv1.EventEnvelope) { e.Type = "employer.unknown" }},
		{"data does not match the routing key", func(e *eventsv1.EventEnvelope) { e.Data = rejectedData }},
		{"missing data", func(e *eventsv1.EventEnvelope) { e.Data = nil }},
		{"missing occurred_at", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }},
		{"event ID not a UUID", func(e *eventsv1.EventEnvelope) { e.EventId = "event-1" }},
		{"employer ID not a UUID", func(e *eventsv1.EventEnvelope) {
			data, err := anypb.New(&eventsv1.EmployerVerified{EmployerId: "employer-1"})
			require.NoError(t, err)
			e.Data = data
		}},
		{"rejection without reason", func(e *eventsv1.EventEnvelope) {
			e.Type = rabbitmq.EventEmployerVerificationRejected
			data, err := anypb.New(&eventsv1.EmployerVerificationRejected{EmployerId: fixtureEmployerID})
			require.NoError(t, err)
			e.Data = data
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _ := newHandler(t) // the mock fails the test if the repository is called
			envelope := loadFixture(t, "employer.verified.json")
			tt.change(envelope)
			err := handler(context.Background(), envelope)
			require.ErrorIs(t, err, events.ErrPermanent)
			require.ErrorIs(t, err, domain.ErrInvalidVerificationEvent)
		})
	}
}

func TestVerificationConsumerConfig_UsesEmployerQueueAndBothEvents(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	cfg := rabbitmq.VerificationConsumerConfig("amqp://broker", logger, nil)
	require.Equal(t, "employer.verification", cfg.Queue)
	require.ElementsMatch(t, []string{"employer.verified", "employer.verification_rejected"}, cfg.Bindings)
	require.Equal(t, "amqp://broker", cfg.URL)
	require.NotNil(t, cfg.Handler)
	require.Same(t, logger, cfg.Logger)
}
