package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/ports"
)

// MaxPages stops a run whose list never ends (100 pages of up to 100 items).
const MaxPages = 100

// PublishTimeout bounds the wait for one broker confirmation.
const PublishTimeout = 10 * time.Second

// Each error names what failed; the HTTP handler logs only this reason.
var (
	ErrJobs    = errors.New("closing jobs unavailable")
	ErrSavers  = errors.New("job savers unavailable")
	ErrPublish = errors.New("reminder publish failed")
)

// Result counts one run. Reminders is how many reminders were built,
// Published how many the broker confirmed, Failed how many jobs or reminders
// failed, and Skipped how many savers had an ID that is not a UUID.
type Result struct {
	Jobs, Reminders, Published, Failed, Skipped int
}

// Runner finds jobs closing within 48 hours and publishes one
// deadline.reminder per saver and job (JT-82).
type Runner struct {
	Jobs      ports.Jobs
	Savers    ports.Savers
	Publisher ports.Publisher
	Now       func() time.Time
}

// Run reads every closing job first; if that fails nothing is published. Then,
// job by job, it reads the savers and publishes their reminders. A failed
// saver list or publish does not stop the other jobs, but the run returns an
// error so the trigger is retried. A retry or the next day's run republishes
// the same event_id for every reminder, so consumers see each reminder once.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	now := r.Now()
	start, end := domain.Period(now)
	var res Result
	jobs, err := r.closingJobs(ctx, start, end)
	if err != nil {
		return res, err
	}
	res.Jobs = len(jobs)
	var failed []error
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return res, errors.Join(append(failed, err)...)
		}
		savers, err := r.savers(ctx, job.ID)
		if err != nil {
			res.Failed++
			failed = appendOnce(failed, ErrSavers)
			continue
		}
		for _, applicantID := range savers {
			reminder, err := domain.NewReminder(job.ID, applicantID, job.Deadline)
			if err != nil {
				res.Skipped++
				continue
			}
			res.Reminders++
			event, err := ReminderEvent(reminder, now)
			if err == nil {
				publishCtx, cancel := context.WithTimeout(ctx, PublishTimeout)
				err = r.Publisher.Publish(publishCtx, event)
				cancel()
			}
			if err != nil {
				res.Failed++
				failed = appendOnce(failed, ErrPublish)
				continue
			}
			res.Published++
		}
	}
	return res, errors.Join(failed...)
}

// closingJobs pages through every closing job; a job seen twice is kept once.
func (r *Runner) closingJobs(ctx context.Context, start, end time.Time) ([]domain.ClosingJob, error) {
	var jobs []domain.ClosingJob
	seen := map[string]bool{}
	err := pages(func(token string) (string, error) {
		batch, next, err := r.Jobs.ListClosing(ctx, start, end, token)
		for _, j := range batch {
			if !seen[j.ID] {
				seen[j.ID] = true
				jobs = append(jobs, j)
			}
		}
		return next, err
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJobs, err)
	}
	return jobs, nil
}

// savers pages through one job's savers; an applicant listed twice is kept once.
func (r *Runner) savers(ctx context.Context, jobID string) ([]string, error) {
	var ids []string
	seen := map[string]bool{}
	err := pages(func(token string) (string, error) {
		batch, next, err := r.Savers.ListSavers(ctx, jobID, token)
		for _, id := range batch {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		return next, err
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSavers, err)
	}
	return ids, nil
}

// pages calls fetch until it returns an empty next token. A repeated token or
// more than MaxPages pages is an error, so a broken upstream cannot loop forever.
func pages(fetch func(token string) (next string, err error)) error {
	token := ""
	seen := map[string]bool{}
	for page := 1; ; page++ {
		if page > MaxPages {
			return fmt.Errorf("more than %d pages", MaxPages)
		}
		next, err := fetch(token)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		if seen[next] {
			return errors.New("page token repeated")
		}
		seen[next] = true
		token = next
	}
}

func appendOnce(errs []error, err error) []error {
	for _, e := range errs {
		if e == err {
			return errs
		}
	}
	return append(errs, err)
}
