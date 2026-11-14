package rabbitmq

import (
	"context"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"log/slog"
)

type ApplicationRecorder interface {
	RecordApplication(context.Context, string, domain.ApplicationParticipants) error
}

func ApplicationConsumerConfig(url string, logger *slog.Logger, recorder ApplicationRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{URL: url, Queue: "offer.applications", Bindings: []string{"application.submitted"}, Logger: logger, Handler: ApplicationHandler(recorder)}
}

func ApplicationHandler(recorder ApplicationRecorder) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		invalid := func() error { return events.Permanent(errors.New("invalid application submitted event")) }
		if _, err := events.MarshalEnvelope(e); err != nil {
			return invalid()
		}
		if e.Type != "application.submitted" || e.Producer != "application" {
			return invalid()
		}
		var data eventsv1.ApplicationSubmitted
		if err := e.Data.UnmarshalTo(&data); err != nil {
			return invalid()
		}
		ids := []string{data.ApplicationId, data.ApplicantId, data.EmployerId, data.JobId, data.ResumeId}
		for i, id := range ids {
			normalized, err := domain.NormalizeID(id, domain.ErrInvalidApplicationID)
			if err != nil {
				return invalid()
			}
			ids[i] = normalized
		}
		return recorder.RecordApplication(ctx, e.EventId, domain.ApplicationParticipants{ApplicationID: ids[0], ApplicantID: ids[1], EmployerID: ids[2]})
	}
}
