package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
)

// EventsQueue is audit-log's durable queue for every domain event, named
// <service>.<purpose> (AGENTS.md Section 9).
const EventsQueue = "audit-log.events"

// AllEvents is the topic binding that matches every routing key
// (audit-log consumes all events, AGENTS.md Section 4).
const AllEvents = events.AllEventsBinding

// EntryRecorder is the use case the handler calls (app.AuditLog).
type EntryRecorder interface {
	RecordEntry(context.Context, app.RecordEntryInput) (bool, error)
}

// EventsConsumerConfig is the pkg/events consumer for all events (JT-72).
func EventsConsumerConfig(rabbitMQURL string, logger *slog.Logger, recorder EntryRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      rabbitMQURL,
		Queue:    EventsQueue,
		Bindings: []string{AllEvents},
		Handler:  EventsHandler(recorder, logger),
		Logger:   logger,
	}
}

// EventsHandler records one event. An event that fails the JT-35 rules goes
// to the dead queue; a database error is retried by the consumer.
func EventsHandler(recorder EntryRecorder, logger *slog.Logger) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		in, err := InputFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		recorded, err := recorder.RecordEntry(ctx, in)
		if errors.Is(err, app.ErrInvalidEntry) {
			return events.Permanent(err)
		}
		if err != nil {
			return err
		}
		logger.Info("audit event handled", "event_id", in.EventID, "event_type", in.EventType, "duplicate", !recorded)
		return nil
	}
}

// InputFromEnvelope copies the envelope into the use case input. The data is
// stored as the protojson of the Any, so its "@type" names the event message.
// Producers are not checked against Section 4: the audit log records what
// was published.
func InputFromEnvelope(envelope *eventsv1.EventEnvelope) (app.RecordEntryInput, error) {
	if envelope.GetData() == nil {
		return app.RecordEntryInput{}, fmt.Errorf("%w: event has no data", app.ErrInvalidEntry)
	}
	data, err := protojson.Marshal(envelope.GetData())
	if err != nil {
		return app.RecordEntryInput{}, fmt.Errorf("%w: event data cannot be encoded", app.ErrInvalidEntry)
	}
	// A missing or invalid timestamp stays zero, which NewEntry rejects.
	var occurredAt time.Time
	if ts := envelope.GetOccurredAt(); ts != nil && ts.CheckValid() == nil {
		occurredAt = ts.AsTime()
	}
	return app.RecordEntryInput{
		EventID:    envelope.GetEventId(),
		EventType:  envelope.GetType(),
		Producer:   envelope.GetProducer(),
		OccurredAt: occurredAt,
		DataJSON:   string(data),
	}, nil
}
