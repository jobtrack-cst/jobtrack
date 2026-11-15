package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

// OutcomeQueue is analytics' durable queue for the events it counts, named
// <service>.<purpose> (AGENTS.md Section 9).
const OutcomeQueue = "analytics.outcomes"

// outcomeSource is the only producer allowed for an event type (AGENTS.md
// Section 4) and the data message that type carries.
type outcomeSource struct {
	producer string
	newData  func() proto.Message
}

var outcomeSources = map[string]outcomeSource{
	domain.EventJobPublished:         {"job-posting", func() proto.Message { return &eventsv1.JobPublished{} }},
	domain.EventJobClosed:            {"job-posting", func() proto.Message { return &eventsv1.JobClosed{} }},
	domain.EventApplicationSubmitted: {"application", func() proto.Message { return &eventsv1.ApplicationSubmitted{} }},
	domain.EventApplicationWithdrawn: {"application", func() proto.Message { return &eventsv1.ApplicationWithdrawn{} }},
	domain.EventStatusChanged:        {"application-status", func() proto.Message { return &eventsv1.StatusChanged{} }},
	domain.EventOfferMade:            {"offer", func() proto.Message { return &eventsv1.OfferMade{} }},
	domain.EventOfferAccepted:        {"offer", func() proto.Message { return &eventsv1.OfferAccepted{} }},
	domain.EventOfferDeclined:        {"offer", func() proto.Message { return &eventsv1.OfferDeclined{} }},
}

// OutcomeBindings are the routing keys the queue binds to, in Section 4 order.
func OutcomeBindings() []string {
	return []string{
		domain.EventJobPublished, domain.EventJobClosed,
		domain.EventApplicationSubmitted, domain.EventApplicationWithdrawn, domain.EventStatusChanged,
		domain.EventOfferMade, domain.EventOfferAccepted, domain.EventOfferDeclined,
	}
}

// OutcomeRecorder is the use case the handler calls (app.Analytics).
type OutcomeRecorder interface {
	RecordEvent(context.Context, app.RecordEventInput) (bool, error)
}

// OutcomeConsumerConfig is the pkg/events consumer for all eight events.
func OutcomeConsumerConfig(rabbitMQURL string, logger *slog.Logger, recorder OutcomeRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      rabbitMQURL,
		Queue:    OutcomeQueue,
		Bindings: OutcomeBindings(),
		Handler:  OutcomeHandler(recorder, logger),
		Logger:   logger,
	}
}

// OutcomeHandler records one event. An event that can never be counted goes
// to the dead queue; a database error is retried by the consumer, and a
// duplicate is acknowledged without a new row.
func OutcomeHandler(recorder OutcomeRecorder, logger *slog.Logger) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		in, err := OutcomeFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		recorded, err := recorder.RecordEvent(ctx, in)
		if errors.Is(err, domain.ErrInvalidOutcomeEvent) {
			return events.Permanent(err)
		}
		if err != nil {
			return err
		}
		logger.Info("outcome event handled", "event_id", in.EventID, "event_type", in.EventType, "recorded", recorded)
		return nil
	}
}

// OutcomeFromEnvelope checks the producer and that the data type matches the
// routing key, so a mislabelled event is never counted. IDs inside the data
// are not checked, because analytics does not store them.
func OutcomeFromEnvelope(envelope *eventsv1.EventEnvelope) (app.RecordEventInput, error) {
	source, ok := outcomeSources[envelope.GetType()]
	if !ok || envelope.GetProducer() != source.producer {
		return app.RecordEventInput{}, domain.ErrInvalidOutcomeEvent
	}
	data := source.newData()
	if err := envelope.GetData().UnmarshalTo(data); err != nil {
		return app.RecordEventInput{}, domain.ErrInvalidOutcomeEvent
	}
	var newStatus string
	if changed, ok := data.(*eventsv1.StatusChanged); ok {
		newStatus = changed.GetNewStatus()
	}
	if envelope.GetOccurredAt() == nil || envelope.GetOccurredAt().CheckValid() != nil {
		return app.RecordEventInput{}, domain.ErrInvalidOutcomeEvent
	}
	return app.RecordEventInput{
		EventID:    envelope.GetEventId(),
		EventType:  envelope.GetType(),
		NewStatus:  newStatus,
		OccurredAt: envelope.GetOccurredAt().AsTime(),
	}, nil
}
