package rabbitmq

import (
	"context"
	"log/slog"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

// JobsQueue is saved-jobs' durable queue for job events, named
// <service>.<purpose> (AGENTS.md Section 9).
const JobsQueue = "saved-jobs.jobs"

// JobClosedProducer is the only service that may publish job.closed.
const JobClosedProducer = "job-posting"

// EventJobClosed is the routing key the queue binds to (AGENTS.md Section 4).
const EventJobClosed = "job.closed"

// JobCloser is the use case the handler calls (app.SavedJobs).
type JobCloser interface {
	CloseJob(context.Context, domain.JobClosure) (domain.ClosureOutcome, error)
}

// JobClosedConsumerConfig is the pkg/events consumer for job.closed (JT-70).
func JobClosedConsumerConfig(rabbitMQURL string, logger *slog.Logger, closer JobCloser) events.ConsumerConfig {
	return events.ConsumerConfig{
		URL:      rabbitMQURL,
		Queue:    JobsQueue,
		Bindings: []string{EventJobClosed},
		Handler:  JobClosedHandler(closer, logger),
		Logger:   logger,
	}
}

// JobClosedHandler applies a job.closed event. An event that can never be
// applied goes to the dead queue; a database error is retried by the consumer.
func JobClosedHandler(closer JobCloser, logger *slog.Logger) events.Handler {
	return func(ctx context.Context, envelope *eventsv1.EventEnvelope) error {
		closure, err := ClosureFromEnvelope(envelope)
		if err != nil {
			return events.Permanent(err)
		}
		outcome, err := closer.CloseJob(ctx, closure)
		if err != nil {
			return err
		}
		logger.Info("job.closed handled", "event_id", closure.EventID, "job_id", closure.JobID,
			"duplicate", outcome.Duplicate, "removed_bookmarks", outcome.RemovedBookmarks)
		return nil
	}
}

// ClosureFromEnvelope accepts only job.closed from job-posting with matching
// data, so a mislabelled event never deletes bookmarks.
func ClosureFromEnvelope(envelope *eventsv1.EventEnvelope) (domain.JobClosure, error) {
	if envelope.GetProducer() != JobClosedProducer || envelope.GetType() != EventJobClosed || envelope.GetData() == nil {
		return domain.JobClosure{}, domain.ErrInvalidJobClosedEvent
	}
	var data eventsv1.JobClosed
	if err := envelope.GetData().UnmarshalTo(&data); err != nil {
		return domain.JobClosure{}, domain.ErrInvalidJobClosedEvent
	}
	if envelope.GetOccurredAt() == nil || envelope.GetOccurredAt().CheckValid() != nil {
		return domain.JobClosure{}, domain.ErrInvalidJobClosedEvent
	}
	return domain.NewJobClosure(envelope.GetEventId(), data.GetJobId(), envelope.GetOccurredAt().AsTime())
}
