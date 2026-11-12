package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// VerificationQueue is employer's durable queue for verification decisions,
// named <service>.<purpose> (AGENTS.md Section 9).
const VerificationQueue = "employer.verification"

// VerificationProducer is the only service that may publish these events.
const VerificationProducer = "company-verification"

// Routing keys the queue binds to (AGENTS.md Section 4).
const (
	EventEmployerVerified             = "employer.verified"
	EventEmployerVerificationRejected = "employer.verification_rejected"
)

// VerificationApplier is the use case the handler calls (app.Verifications).
type VerificationApplier interface {
	Apply(context.Context, domain.VerificationDecision) (domain.VerificationOutcome, error)
}

// VerificationConsumerConfig is the pkg/events consumer for both events.
func VerificationConsumerConfig(rabbitMQURL string, logger *slog.Logger, applier VerificationApplier) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      rabbitMQURL,
		Queue:    VerificationQueue,
		Bindings: []string{EventEmployerVerified, EventEmployerVerificationRejected},
		Handler:  VerificationHandler(applier, logger),
		Logger:   logger,
	}
}

// VerificationHandler converts an envelope to a domain decision and applies
// it. An event that can never be applied goes to the dead queue; a database
// error is retried by the consumer.
func VerificationHandler(applier VerificationApplier, logger *slog.Logger) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		decision, err := DecisionFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		outcome, err := applier.Apply(ctx, decision)
		if errors.Is(err, domain.ErrInvalidVerificationEvent) {
			return events.Permanent(err)
		}
		if err != nil {
			return err
		}
		logger.Info("verification event handled",
			"event_id", decision.EventID, "event_type", envelope.GetType(), "outcome", outcome.String())
		return nil
	}
}

// DecisionFromEnvelope reads either verification event. The producer must be
// company-verification and the data type must match the routing key, so a
// mislabelled event is never applied.
func DecisionFromEnvelope(envelope *eventsv1.EventEnvelope) (domain.VerificationDecision, error) {
	if envelope.GetProducer() != VerificationProducer {
		return domain.VerificationDecision{}, domain.ErrInvalidVerificationEvent
	}
	var employerID, reason string
	var status domain.VerificationStatus
	switch envelope.GetType() {
	case EventEmployerVerified:
		var data eventsv1.EmployerVerified
		if err := envelope.GetData().UnmarshalTo(&data); err != nil {
			return domain.VerificationDecision{}, domain.ErrInvalidVerificationEvent
		}
		employerID, status = data.GetEmployerId(), domain.VerificationVerified
	case EventEmployerVerificationRejected:
		var data eventsv1.EmployerVerificationRejected
		if err := envelope.GetData().UnmarshalTo(&data); err != nil {
			return domain.VerificationDecision{}, domain.ErrInvalidVerificationEvent
		}
		employerID, reason, status = data.GetEmployerId(), data.GetReason(), domain.VerificationRejected
	default:
		return domain.VerificationDecision{}, domain.ErrInvalidVerificationEvent
	}
	if envelope.GetOccurredAt() == nil || envelope.GetOccurredAt().CheckValid() != nil {
		return domain.VerificationDecision{}, domain.ErrInvalidVerificationEvent
	}
	return domain.NewVerificationDecision(envelope.GetEventId(), employerID, status, reason, envelope.GetOccurredAt().AsTime())
}
