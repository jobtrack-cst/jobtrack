package app_test

import (
	"context"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testOutbox struct {
	event    *eventsv1.EventEnvelope
	marked   bool
	failMark bool
}

func (r *testOutbox) Pending(context.Context, int) ([]*eventsv1.EventEnvelope, error) {
	if r.marked {
		return nil, nil
	}
	return []*eventsv1.EventEnvelope{r.event}, nil
}
func (r *testOutbox) MarkPublished(context.Context, string, time.Time) error {
	if r.failMark {
		return errors.New("mark failed")
	}
	r.marked = true
	return nil
}

type testPublisher struct {
	fail bool
	ids  []string
}

func (p *testPublisher) Publish(ctx context.Context, e *eventsv1.EventEnvelope) error {
	if _, ok := ctx.Deadline(); !ok {
		panic("missing publish deadline")
	}
	p.ids = append(p.ids, e.EventId)
	if p.fail {
		return errors.New("broker down")
	}
	return nil
}

type relayClock struct{}

func (relayClock) Now() time.Time { return time.Now().UTC() }
func TestRelay_RetriesSameIDAfterPublishOrMarkFailure(t *testing.T) {
	for _, failure := range []string{"publish", "mark"} {
		t.Run(failure, func(t *testing.T) {
			r := &testOutbox{event: &eventsv1.EventEnvelope{EventId: "retained-event-id"}, failMark: failure == "mark"}
			p := &testPublisher{fail: failure == "publish"}
			relay := app.NewOutboxRelay(r, p, relayClock{})
			n, err := relay.PublishPending(context.Background())
			require.Error(t, err)
			require.Zero(t, n)
			require.False(t, r.marked)
			r.failMark = false
			p.fail = false
			n, err = relay.PublishPending(context.Background())
			require.NoError(t, err)
			require.Equal(t, 1, n)
			require.Equal(t, []string{"retained-event-id", "retained-event-id"}, p.ids)
			n, err = relay.PublishPending(context.Background())
			require.NoError(t, err)
			require.Zero(t, n)
		})
	}
}
