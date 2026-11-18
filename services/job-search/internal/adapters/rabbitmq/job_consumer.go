package rabbitmq

import (
	"context"
	"errors"
	"log/slog"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/app"
)

func JobConsumerConfig(url string, logger *slog.Logger, consumer *app.Consumer) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      url,
		Queue:    "job-search.jobs",
		Bindings: []string{"job.published", "job.updated", "job.closed"},
		Logger:   logger,
		Handler:  JobHandler(consumer),
	}
}

func JobHandler(consumer *app.Consumer) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		var data interface{}
		var err error

		switch e.Type {
		case "job.published":
			var pub eventsv1.JobPublished
			err = e.Data.UnmarshalTo(&pub)
			data = &pub
		case "job.updated":
			var upd eventsv1.JobUpdated
			err = e.Data.UnmarshalTo(&upd)
			data = &upd
		case "job.closed":
			var closed eventsv1.JobClosed
			err = e.Data.UnmarshalTo(&closed)
			data = &closed
		default:
			return events.Permanent(errors.New("unknown event type"))
		}

		if err != nil {
			return events.Permanent(errors.New("invalid event payload"))
		}

		return consumer.ProcessEvent(ctx, e.EventId, e.Type, e.OccurredAt.AsTime(), data)
	}
}
