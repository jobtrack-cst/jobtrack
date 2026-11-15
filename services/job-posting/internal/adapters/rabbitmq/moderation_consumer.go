package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
)

type ModerationRecorder interface {
	RecordJobRemoved(ctx context.Context, eventID string, jobID string) error
}

func ModerationConsumerConfig(url string, logger *slog.Logger, recorder ModerationRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      url,
		Queue:    "job-posting.moderation",
		Bindings: []string{"moderation.job_removed"},
		Logger:   logger,
		Handler:  ModerationHandler(recorder),
	}
}

func ModerationHandler(recorder ModerationRecorder) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		invalid := func() error { return events.Permanent(errors.New("invalid moderation.job_removed event")) }
		if _, err := events.MarshalEnvelope(e); err != nil {
			return invalid()
		}
		if e.Type != "moderation.job_removed" || e.Producer != "moderation" {
			return invalid()
		}
		var data eventsv1.ModerationJobRemoved
		if err := e.Data.UnmarshalTo(&data); err != nil {
			return invalid()
		}
		if _, err := uuid.Parse(data.JobId); err != nil {
			return invalid()
		}
		return recorder.RecordJobRemoved(ctx, e.EventId, data.JobId)
	}
}
