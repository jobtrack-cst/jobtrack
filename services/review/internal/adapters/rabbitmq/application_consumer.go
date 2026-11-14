package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

// ApplicationQueue is review's durable queue for application.submitted
// (AGENTS.md Section 9 naming, JT-79).
const ApplicationQueue = "review.applications"

// ApplicationRecorder stores the participants of one application.submitted
// event idempotently (postgres.Participants.RecordApplication).
type ApplicationRecorder interface {
	RecordApplication(ctx context.Context, eventID string, p domain.ApplicationParticipants) error
}

// ApplicationConsumerConfig binds only application.submitted. The shared
// events.Consume applies the JT-61 retry and dead-letter policy.
func ApplicationConsumerConfig(url string, logger *slog.Logger, recorder ApplicationRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL: url, Queue: ApplicationQueue, Bindings: []string{"application.submitted"},
		Logger: logger, Handler: ApplicationHandler(recorder),
	}
}

// ApplicationHandler checks the event, then records it. An event that can
// never be applied (bad envelope, producer, type, payload or ID, or
// participants that differ from those stored) goes to the dead queue at once;
// any other error, such as the database being down, is retried.
func ApplicationHandler(recorder ApplicationRecorder) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		invalid := events.Permanent(errors.New("invalid application.submitted event"))
		if _, err := events.MarshalEnvelope(e); err != nil {
			return invalid
		}
		if e.GetType() != "application.submitted" || e.GetProducer() != "application" {
			return invalid
		}
		var data eventsv1.ApplicationSubmitted
		if err := e.GetData().UnmarshalTo(&data); err != nil {
			return invalid
		}
		ids := []string{data.GetApplicationId(), data.GetApplicantId(), data.GetEmployerId(), data.GetJobId(), data.GetResumeId()}
		for i, id := range ids {
			normalized, err := domain.NormalizeApplicationID(id)
			if err != nil {
				return invalid
			}
			ids[i] = normalized
		}
		err := recorder.RecordApplication(ctx, e.GetEventId(),
			domain.ApplicationParticipants{ApplicationID: ids[0], ApplicantID: ids[1], EmployerID: ids[2]})
		if errors.Is(err, domain.ErrConflictingParticipants) {
			return events.Permanent(err)
		}
		return err
	}
}
