package app_test

import (
	"context"
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
	"time"
)

func TestOutboxRelay_ConfirmedPublishAndRecovery(t *testing.T) {
	for _, failure := range []string{"none", "pending", "publish", "mark"} {
		t.Run(failure, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOutboxRepository(ctrl)
			pub := mocks.NewMockEventPublisher(ctrl)
			event := &eventsv1.EventEnvelope{EventId: "original-ID"}
			now := time.Now().UTC()
			boom := errors.New("unavailable")
			relay := app.NewOutboxRelay(repo, pub, func() time.Time { return now })
			if failure == "pending" {
				repo.EXPECT().Pending(gomock.Any(), 50).Return(nil, boom)
			} else {
				repo.EXPECT().Pending(gomock.Any(), 50).Return([]*eventsv1.EventEnvelope{event}, nil)
				call := pub.EXPECT().Publish(gomock.Any(), event).DoAndReturn(func(ctx context.Context, _ *eventsv1.EventEnvelope) error {
					_, ok := ctx.Deadline()
					require.True(t, ok)
					if failure == "publish" {
						return boom
					}
					return nil
				})
				if failure != "publish" {
					repo.EXPECT().MarkPublished(gomock.Any(), event.EventId, now).After(call).DoAndReturn(func(context.Context, string, time.Time) error {
						if failure == "mark" {
							return boom
						}
						return nil
					})
				}
			}
			n, err := relay.PublishPending(context.Background())
			if failure == "none" {
				require.NoError(t, err)
				require.Equal(t, 1, n)
			} else {
				require.ErrorIs(t, err, boom)
				require.Zero(t, n)
				repo.EXPECT().Pending(gomock.Any(), 50).Return([]*eventsv1.EventEnvelope{event}, nil)
				pub.EXPECT().Publish(gomock.Any(), event).Return(nil)
				repo.EXPECT().MarkPublished(gomock.Any(), event.EventId, now).Return(nil)
				n, err = relay.PublishPending(context.Background())
				require.NoError(t, err)
				require.Equal(t, 1, n)
			}
		})
	}
}
