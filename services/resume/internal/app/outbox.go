package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

const (
	OutboxLease          = 30 * time.Second
	OutboxPublishTimeout = 10 * time.Second
	OutboxIdleWait       = time.Second
	OutboxMaxBackoff     = 30 * time.Second
)

// OutboxDispatcher is owned by one synchronous worker. Step and Run must not
// be called concurrently. The repository returns committed, unlocked claims.
type OutboxDispatcher struct {
	repo      ports.OutboxRepository
	publisher ports.EventPublisher
	logger    *slog.Logger
	backoff   time.Duration
}

func NewOutboxDispatcher(repo ports.OutboxRepository, publisher ports.EventPublisher, logger *slog.Logger) *OutboxDispatcher {
	return &OutboxDispatcher{repo: repo, publisher: publisher, logger: logger}
}

func (d *OutboxDispatcher) failedDelay() time.Duration {
	if d.backoff == 0 {
		d.backoff = time.Second
	} else {
		d.backoff = min(d.backoff*2, OutboxMaxBackoff)
	}
	return d.backoff
}

// Step makes at most one publication and returns the delay before the next poll.
// All error messages are fixed; never surface persisted envelopes or driver errors.
func (d *OutboxDispatcher) Step(ctx context.Context) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	event, found, err := d.repo.Claim(ctx, OutboxLease)
	if err != nil {
		return d.failedDelay(), errors.New("claim outgoing event failed")
	}
	if !found {
		return OutboxIdleWait, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	envelope, err := events.UnmarshalEnvelope([]byte(event.Payload))
	if err == nil {
		payload := new(eventsv1.ResumeUploaded)
		if envelope.EventId != event.ID || event.Type != "resume.uploaded" || envelope.Type != event.Type || envelope.Producer != "resume" ||
			envelope.Data.UnmarshalTo(payload) != nil || uuid.Validate(payload.ResumeId) != nil || uuid.Validate(payload.ApplicantId) != nil {
			err = errors.New("invalid stored uploaded event")
		}
	}
	if err != nil {
		return d.retry(ctx, event.ID, event.ClaimToken, "invalid stored uploaded event", nil)
	}
	publishCtx, cancel := context.WithTimeout(ctx, OutboxPublishTimeout)
	err = d.publisher.Publish(publishCtx, envelope)
	cancel()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	} // Leave ownership to expire on shutdown.
	if err != nil {
		return d.retry(ctx, event.ID, event.ClaimToken, "publish outgoing event failed", err)
	}
	changed, err := d.repo.MarkPublished(ctx, event.ID, event.ClaimToken)
	if err != nil {
		d.log("record outgoing confirmation failed", event.ID)
		return d.failedDelay(), errors.New("record outgoing confirmation failed")
	}
	if !changed {
		d.log("outgoing event ownership lost", event.ID)
		return OutboxIdleWait, nil
	}
	d.backoff = 0
	return 0, nil
}

func (d *OutboxDispatcher) retry(ctx context.Context, id, token, message string, cause error) (time.Duration, error) {
	// Invalid saved events are event-specific: defer them without delaying
	// unrelated claims or advancing the worker's transport/DB failure backoff.
	delay := OutboxMaxBackoff
	if cause != nil {
		delay = d.failedDelay()
	}
	d.log(message, id)
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	changed, err := d.repo.Retry(ctx, id, token, delay)
	if errors.Is(cause, ports.ErrPublisherUnusable) {
		return delay, ports.ErrPublisherUnusable
	}
	if err != nil {
		if cause == nil {
			delay = d.failedDelay()
		}
		return delay, errors.New("reschedule outgoing event failed")
	}
	if !changed {
		d.log("outgoing event ownership lost", id)
		if cause == nil {
			return OutboxIdleWait, errors.New(message)
		}
	}
	if cause == nil {
		return 0, errors.New(message)
	}
	return delay, errors.New(message)
}

func (d *OutboxDispatcher) log(message, id string) {
	if d.logger != nil {
		d.logger.Warn(message, "event_id", id)
	}
}

// Run never spawns per-attempt goroutines. Cancellation interrupts waits and
// confirmation waiting, but cannot interrupt shared publisher socket I/O/Close.
func (d *OutboxDispatcher) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		delay, err := d.Step(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, ports.ErrPublisherUnusable) {
			return ports.ErrPublisherUnusable
		}
		if err != nil && d.logger != nil {
			d.logger.Warn("outbox attempt failed", "error", err)
		}
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
	}
	return nil
}
