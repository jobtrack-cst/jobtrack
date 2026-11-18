package rabbitmq

import (
	"context"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"log/slog"
	"strings"
	"unicode/utf8"
)

type JobRecorder interface {
	RecordJob(context.Context, string, *eventsv1.JobPublished) error
}

func JobConsumerConfig(url string, logger *slog.Logger, recorder JobRecorder) events.ConsumerConfig {
	return events.ConsumerConfig{URL: url, Queue: "job-alert.jobs", Bindings: []string{"job.published"}, Logger: logger, Handler: JobHandler(recorder)}
}
func JobHandler(recorder JobRecorder) events.Handler {
	return func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		invalid := func() error { return events.Permanent(errors.New("invalid job published event")) }
		if _, err := events.MarshalEnvelope(e); err != nil {
			return invalid()
		}
		if e.Type != "job.published" || e.Producer != "job-posting" {
			return invalid()
		}
		var job eventsv1.JobPublished
		if err := e.Data.UnmarshalTo(&job); err != nil {
			return invalid()
		}
		for _, id := range append([]string{job.JobId, job.EmployerId}, job.TagIds...) {
			if _, err := domain.NormalizeID(id, domain.ErrInvalid); err != nil {
				return invalid()
			}
		}
		if job.CategoryId != "" {
			if _, err := domain.NormalizeID(job.CategoryId, domain.ErrInvalid); err != nil {
				return invalid()
			}
		}
		if job.Deadline == nil || job.Deadline.CheckValid() != nil || strings.TrimSpace(job.Title) == "" || !utf8.ValidString(job.Location) || strings.ContainsRune(job.Location, 0) {
			return invalid()
		}
		job.JobId = strings.ToLower(job.JobId)
		job.CategoryId = strings.ToLower(job.CategoryId)
		for i := range job.TagIds {
			job.TagIds[i] = strings.ToLower(job.TagIds[i])
		}
		return recorder.RecordJob(ctx, strings.ToLower(e.EventId), &job)
	}
}
