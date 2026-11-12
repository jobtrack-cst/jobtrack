package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

const ApplicationQueue = "application-status.applications"

func ApplicationConsumerConfig(url string, logger *slog.Logger, repo ports.ApplicationEventRepository) events.ConsumerConfig {
	return events.ConsumerConfig{URL: url, Queue: ApplicationQueue, Bindings: []string{"application.submitted", "application.withdrawn"}, Logger: logger, Handler: ApplicationHandler(repo)}
}

func ApplicationHandler(repo ports.ApplicationEventRepository) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		input, err := ApplicationFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		err = repo.ApplyEvent(ctx, input)
		if errors.Is(err, domain.ErrConflictingEvent) || errors.Is(err, domain.ErrInvalidTransition) {
			return events.Permanent(err)
		}
		return err
	}
}

func ApplicationFromEnvelope(e *eventsv1.EventEnvelope) (domain.ApplicationEvent, error) {
	bad := domain.ErrConflictingEvent
	if e.GetProducer() != "application" || e.GetOccurredAt() == nil || e.GetOccurredAt().CheckValid() != nil || e.GetOccurredAt().AsTime().IsZero() {
		return domain.ApplicationEvent{}, bad
	}
	if _, err := uuid.Parse(e.GetEventId()); err != nil || len(e.GetEventId()) != 36 {
		return domain.ApplicationEvent{}, bad
	}
	v := domain.ApplicationEvent{EventID: e.GetEventId(), OccurredAt: e.GetOccurredAt().AsTime().UTC()}
	var ids []string
	switch e.GetType() {
	case "application.submitted":
		d := new(eventsv1.ApplicationSubmitted)
		if e.GetData() == nil || e.GetData().UnmarshalTo(d) != nil {
			return v, bad
		}
		v.ApplicationID, v.ApplicantID, v.EmployerID, v.Submitted = d.ApplicationId, d.ApplicantId, d.EmployerId, true
		ids = []string{v.ApplicationID, v.ApplicantID, v.EmployerID, d.JobId, d.ResumeId}
	case "application.withdrawn":
		d := new(eventsv1.ApplicationWithdrawn)
		if e.GetData() == nil || e.GetData().UnmarshalTo(d) != nil {
			return v, bad
		}
		v.ApplicationID, v.ApplicantID = d.ApplicationId, d.ApplicantId
		ids = []string{v.ApplicationID, v.ApplicantID, d.JobId}
	default:
		return v, bad
	}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
			return v, bad
		}
	}
	return v, nil
}
