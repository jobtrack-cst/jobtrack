package app

import (
	"context"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/ports"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Consumer struct {
	repo ports.WriteRepository
}

func NewConsumer(repo ports.WriteRepository) *Consumer {
	return &Consumer{repo: repo}
}

// ProcessEvent handles job.published, job.updated, and job.closed events.
func (c *Consumer) ProcessEvent(ctx context.Context, eventID, routingKey string, occurredAt time.Time, data interface{}) error {
	// Idempotency check
	processed, err := c.repo.IsProcessed(ctx, eventID)
	if err != nil {
		return err
	}
	if processed {
		return nil // Ignore duplicate
	}

	switch routingKey {
	case "job.published", "job.updated":
		var pub *eventsv1.JobPublished
		var upd *eventsv1.JobUpdated

		if p, ok := data.(*eventsv1.JobPublished); ok {
			pub = p
		} else if u, ok := data.(*eventsv1.JobUpdated); ok {
			upd = u
		} else {
			return nil // Invalid type
		}

		var summary domain.JobSummary
		if pub != nil {
			summary = domain.JobSummary{
				JobID:      pub.JobId,
				EmployerID: pub.EmployerId,
				Title:      pub.Title,
				CategoryID: pub.CategoryId,
				TagIDs:     pub.TagIds,
				Location:   pub.Location,
				Deadline:   asTime(pub.Deadline),
			}
		} else {
			summary = domain.JobSummary{
				JobID:      upd.JobId,
				EmployerID: upd.EmployerId,
				Title:      upd.Title,
				CategoryID: upd.CategoryId,
				TagIDs:     upd.TagIds,
				Location:   upd.Location,
				Deadline:   asTime(upd.Deadline),
			}
		}

		if err := c.repo.UpsertJob(ctx, summary, occurredAt); err != nil {
			return err
		}

	case "job.closed":
		closed, ok := data.(*eventsv1.JobClosed)
		if !ok {
			return nil
		}
		if err := c.repo.CloseJob(ctx, closed.JobId, occurredAt); err != nil {
			return err
		}
	}

	return c.repo.MarkProcessed(ctx, eventID)
}

func asTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
