package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports/mocks"
)

type relayDeps struct {
	outbox    *mocks.MockOutboxRepository
	publisher *mocks.MockEventPublisher
	clock     *mocks.MockClock
}

func newRelay(t *testing.T) (*app.OutboxRelay, relayDeps) {
	ctrl := gomock.NewController(t)
	d := relayDeps{
		outbox:    mocks.NewMockOutboxRepository(ctrl),
		publisher: mocks.NewMockEventPublisher(ctrl),
		clock:     mocks.NewMockClock(ctrl),
	}
	return app.NewOutboxRelay(d.outbox, d.publisher, d.clock), d
}

func outboxEvents(t *testing.T, ids ...string) []*eventsv1.EventEnvelope {
	t.Helper()
	r := stored(domain.StatusApproved)
	r.ReviewedBy, r.UpdatedAt = adminID, now
	out := make([]*eventsv1.EventEnvelope, 0, len(ids))
	for _, id := range ids {
		e, err := app.DecisionEvent(r, id)
		require.NoError(t, err)
		out = append(out, e)
	}
	return out
}

const (
	firstEventID  = "a1000000-0000-4000-8000-000000000001"
	secondEventID = "a1000000-0000-4000-8000-000000000002"
)

func TestOutboxRelay_PublishPending(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   func(d relayDeps, pending []*eventsv1.EventEnvelope)
		want    int
		wantErr error
	}{
		{
			name: "publishes every pending event in order and marks each one",
			setup: func(d relayDeps, pending []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(pending, nil)
				d.clock.EXPECT().Now().Return(now).Times(2)
				gomock.InOrder(
					d.publisher.EXPECT().Publish(gomock.Any(), pending[0]).Return(nil),
					d.outbox.EXPECT().MarkPublished(gomock.Any(), firstEventID, now).Return(nil),
					d.publisher.EXPECT().Publish(gomock.Any(), pending[1]).Return(nil),
					d.outbox.EXPECT().MarkPublished(gomock.Any(), secondEventID, now).Return(nil),
				)
			},
			want: 2,
		},
		{
			name: "nothing pending",
			setup: func(d relayDeps, _ []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(nil, nil)
			},
			want: 0,
		},
		{
			name: "read error is returned",
			setup: func(d relayDeps, _ []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(nil, errDB)
			},
			wantErr: errDB,
		},
		{
			name: "publish failure stops before later events and leaves the event unmarked",
			setup: func(d relayDeps, pending []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(pending, nil)
				d.publisher.EXPECT().Publish(gomock.Any(), pending[0]).Return(errDB)
			},
			want: 0, wantErr: errDB,
		},
		{
			name: "mark failure after a confirmed publish stops the batch",
			setup: func(d relayDeps, pending []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(pending, nil)
				d.clock.EXPECT().Now().Return(now)
				d.publisher.EXPECT().Publish(gomock.Any(), pending[0]).Return(nil)
				d.outbox.EXPECT().MarkPublished(gomock.Any(), firstEventID, now).Return(errDB)
			},
			want: 0, wantErr: errDB,
		},
		{
			name: "second publish failure keeps the first as published",
			setup: func(d relayDeps, pending []*eventsv1.EventEnvelope) {
				d.outbox.EXPECT().Pending(gomock.Any(), app.OutboxBatchSize).Return(pending, nil)
				d.clock.EXPECT().Now().Return(now)
				d.publisher.EXPECT().Publish(gomock.Any(), pending[0]).Return(nil)
				d.outbox.EXPECT().MarkPublished(gomock.Any(), firstEventID, now).Return(nil)
				d.publisher.EXPECT().Publish(gomock.Any(), pending[1]).Return(errDB)
			},
			want: 1, wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			relay, d := newRelay(t)
			tt.setup(d, outboxEvents(t, firstEventID, secondEventID))
			got, err := relay.PublishPending(context.Background())
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}
