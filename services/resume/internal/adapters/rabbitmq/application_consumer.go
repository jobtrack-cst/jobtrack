package rabbitmq

import (
	"context"
	"log/slog"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// ApplicationQueue holds application.submitted for the JT-114 projection.
// events.Consume also declares ApplicationQueue + ".dead".
const ApplicationQueue = "resume.applications"

// ApplicationConsumerConfig uses the shared events.Consume (JT-56/JT-61
// policy). It is separate from, and does not change, the JT-59 validation
// consumer.
func ApplicationConsumerConfig(url string, logger *slog.Logger, repo ports.ApplicationResumeRepository) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL: url, Queue: ApplicationQueue, Bindings: []string{"application.submitted"},
		Logger: logger, Handler: ApplicationHandler(repo),
	}
}

// ApplicationHandler applies the JT-79 rules: anything malformed goes to the
// dead queue at once; database errors from the repository are retried.
func ApplicationHandler(repo ports.ApplicationResumeRepository) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		invalid := events.Permanent(domain.ErrInvalidApplicationEvent)
		if _, err := events.MarshalEnvelope(e); err != nil {
			return invalid
		}
		if e.GetType() != "application.submitted" || e.GetProducer() != "application" {
			return invalid
		}
		eventID, err := domain.NormalizeEventID(e.GetEventId())
		if err != nil {
			return invalid
		}
		var data eventsv1.ApplicationSubmitted
		if err := e.GetData().UnmarshalTo(&data); err != nil {
			return invalid
		}
		row, err := domain.NewApplicationResume(data.GetApplicationId(), data.GetApplicantId(), data.GetEmployerId(), data.GetResumeId())
		if err != nil {
			return invalid
		}
		return repo.Record(ctx, eventID, row)
	}
}
