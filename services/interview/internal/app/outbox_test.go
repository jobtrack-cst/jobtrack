package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/ports/mocks"
)

var relayNow = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)

type relayFixture struct {
	relay     *app.OutboxRelay
	outbox    *mocks.MockOutboxRepository
	publisher *mocks.MockEventPublisher
}

func newRelay(t *testing.T) relayFixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(relayNow).AnyTimes()
	f := relayFixture{outbox: mocks.NewMockOutboxRepository(ctrl), publisher: mocks.NewMockEventPublisher(ctrl)}
	f.relay = app.NewOutboxRelay(f.outbox, f.publisher, clock)
	return f
}

func envelopes(ids ...string) []*eventsv1.EventEnvelope {
	out := make([]*eventsv1.EventEnvelope, len(ids))
	for i, id := range ids {
		out[i] = &eventsv1.EventEnvelope{EventId: id}
	}
	return out
}

// Each publish has its own deadline, so a silent broker cannot stall the relay.
func expectPublish(f relayFixture, id string, err error) *gomock.Call {
	return f.publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > app.PublishTimeout {
			return errors.New("publish without the 10 s deadline")
		}
		if e.GetEventId() != id {
			return errors.New("published " + e.GetEventId() + ", want " + id)
		}
		return err
	})
}

func TestOutboxRelay_PublishesInOrderAndMarks(t *testing.T) {
	f := newRelay(t)
	f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(envelopes("e1", "e2"), nil)
	gomock.InOrder(
		expectPublish(f, "e1", nil),
		f.outbox.EXPECT().MarkPublished(gomock.Any(), "e1", relayNow).Return(nil),
		expectPublish(f, "e2", nil),
		f.outbox.EXPECT().MarkPublished(gomock.Any(), "e2", relayNow).Return(nil),
	)
	n, err := f.relay.PublishPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestOutboxRelay_EmptyOutbox(t *testing.T) {
	f := newRelay(t)
	f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(nil, nil)
	n, err := f.relay.PublishPending(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestOutboxRelay_StopsAtFirstFailure(t *testing.T) {
	errBroker := errors.New("broker down")
	errMark := errors.New("mark failed")
	errRead := errors.New("read failed")
	t.Run("read fails", func(t *testing.T) {
		f := newRelay(t)
		f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(nil, errRead)
		n, err := f.relay.PublishPending(context.Background())
		require.ErrorIs(t, err, errRead)
		assert.Zero(t, n)
	})
	t.Run("publish fails: not marked, later events wait", func(t *testing.T) {
		f := newRelay(t)
		f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(envelopes("e1", "e2", "e3"), nil)
		gomock.InOrder(
			expectPublish(f, "e1", nil),
			f.outbox.EXPECT().MarkPublished(gomock.Any(), "e1", relayNow).Return(nil),
			expectPublish(f, "e2", errBroker),
		)
		n, err := f.relay.PublishPending(context.Background())
		require.ErrorIs(t, err, errBroker)
		assert.Equal(t, 1, n)
	})
	t.Run("mark fails after a confirmed publish", func(t *testing.T) {
		f := newRelay(t)
		f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(envelopes("e1", "e2"), nil)
		gomock.InOrder(
			expectPublish(f, "e1", nil),
			f.outbox.EXPECT().MarkPublished(gomock.Any(), "e1", relayNow).Return(errMark),
		)
		n, err := f.relay.PublishPending(context.Background())
		require.ErrorIs(t, err, errMark)
		assert.Zero(t, n)
	})
}

// After a failure the next call publishes the same event ID again; consumers
// ignore the repeat by event_id.
func TestOutboxRelay_RetriesSameEventID(t *testing.T) {
	f := newRelay(t)
	gomock.InOrder(
		f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(envelopes("e1"), nil),
		expectPublish(f, "e1", nil),
		f.outbox.EXPECT().MarkPublished(gomock.Any(), "e1", relayNow).Return(errors.New("mark failed")),
		f.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(envelopes("e1"), nil),
		expectPublish(f, "e1", nil),
		f.outbox.EXPECT().MarkPublished(gomock.Any(), "e1", relayNow).Return(nil),
	)
	_, err := f.relay.PublishPending(context.Background())
	require.Error(t, err)
	n, err := f.relay.PublishPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
