package main

import (
	"context"
	"errors"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/app"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

type stubJobs struct{ jobs []domain.ClosingJob }

func (s stubJobs) ListClosing(context.Context, time.Time, time.Time, string) ([]domain.ClosingJob, string, error) {
	return s.jobs, "", nil
}

type stubSavers struct{ ids []string }

func (s stubSavers) ListSavers(context.Context, string, string) ([]string, string, error) {
	return s.ids, "", nil
}

type countingPublisher struct {
	published []string
	closed    int
}

func (p *countingPublisher) Publish(_ context.Context, e *eventsv1.EventEnvelope) error {
	p.published = append(p.published, e.GetEventId())
	return nil
}

func (p *countingPublisher) Close() error { p.closed++; return nil }

var runAt = time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)

// Each run opens its own broker connection and closes it, even when nothing
// is published.
func TestPublishingRunner_OpensAndClosesAConnectionPerRun(t *testing.T) {
	pub := &countingPublisher{}
	opened := []string{}
	r := publishingRunner{
		jobs:   stubJobs{jobs: []domain.ClosingJob{{ID: "33333333-3333-4333-8333-333333333333", Deadline: runAt.Add(time.Hour)}}},
		savers: stubSavers{ids: []string{"22222222-2222-4222-8222-222222222222"}},
		open:   func(url string) (publisherResource, error) { opened = append(opened, url); return pub, nil },
		url:    "amqp://synthetic:synthetic@rabbitmq:5672/",
		now:    func() time.Time { return runAt },
	}
	for range 2 {
		res, err := r.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, app.Result{Jobs: 1, Reminders: 1, Published: 1}, res)
	}
	assert.Equal(t, []string{r.url, r.url}, opened)
	assert.Equal(t, 2, pub.closed)
	require.Len(t, pub.published, 2)
	assert.Equal(t, pub.published[0], pub.published[1], "a second run republishes the same event_id")
}

func TestPublishingRunner_BrokerUnavailable(t *testing.T) {
	r := publishingRunner{
		jobs: stubJobs{}, savers: stubSavers{},
		open: func(string) (publisherResource, error) {
			return nil, errors.New("dial amqp://synthetic:private-secret@rabbitmq:5672/ failed")
		},
		now: func() time.Time { return runAt },
	}
	_, err := r.Run(context.Background())
	require.ErrorIs(t, err, app.ErrPublish)
	assert.NotContains(t, err.Error(), "private-secret", "the broker URL is never in the error")
}
