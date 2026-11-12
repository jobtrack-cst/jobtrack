package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func outgoing(t *testing.T) domain.OutgoingEvent {
	t.Helper()
	data, err := anypb.New(&eventsv1.ResumeUploaded{ResumeId: resumeID, ApplicantId: ownerID})
	require.NoError(t, err)
	envelope := &eventsv1.EventEnvelope{EventId: "11111111-1111-4111-8111-111111111111", Type: "resume.uploaded", Producer: "resume", OccurredAt: timestamppb.New(now), Data: data}
	body, err := events.MarshalEnvelope(envelope)
	require.NoError(t, err)
	return domain.OutgoingEvent{ID: envelope.EventId, Type: envelope.Type, Payload: string(body), ClaimToken: "22222222-2222-4222-8222-222222222222", LeaseUntil: now.Add(time.Minute)}
}

func TestOutboxDispatcher_Step(t *testing.T) {
	for _, scenario := range []string{"empty", "claim error", "success", "publish error", "publish timeout", "malformed", "wrong identity", "wrong type", "wrong payload", "stale completion", "completion error", "retry error", "stale retry", "canceled before claim", "canceled after claim", "canceled publish", "fatal cleanup"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOutboxRepository(ctrl)
			publisher := mocks.NewMockEventPublisher(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			event := outgoing(t)
			wantDelay := time.Second
			wantError := true
			switch scenario {
			case "malformed":
				event.Payload = "private-payload"
			case "wrong identity":
				event.ID = "33333333-3333-4333-8333-333333333333"
			case "wrong type":
				event.Type = "resume.rejected"
			case "wrong payload":
				event.Payload = strings.ReplaceAll(event.Payload, resumeID, "bad")
			}
			if scenario == "canceled before claim" {
				cancel()
				wantDelay = 0
			} else {
				if scenario == "empty" {
					repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(domain.OutgoingEvent{}, false, nil)
					wantError = false
				} else if scenario == "claim error" {
					repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(domain.OutgoingEvent{}, false, errors.New("private-driver"))
				} else {
					repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).DoAndReturn(func(context.Context, time.Duration) (domain.OutgoingEvent, bool, error) {
						if scenario == "canceled after claim" {
							cancel()
						}
						return event, true, nil
					})
					if scenario == "canceled after claim" {
						wantDelay = 0
					} else {
						malformed := scenario == "malformed" || scenario == "wrong identity" || scenario == "wrong type" || scenario == "wrong payload"
						if !malformed {
							publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(c context.Context, e *eventsv1.EventEnvelope) error {
								require.Equal(t, event.ID, e.EventId)
								require.Equal(t, "resume", e.Producer)
								deadline, ok := c.Deadline()
								require.True(t, ok)
								require.LessOrEqual(t, time.Until(deadline), app.OutboxPublishTimeout)
								switch scenario {
								case "publish error", "retry error", "stale retry":
									return errors.New("private-transport")
								case "publish timeout":
									return context.DeadlineExceeded
								case "canceled publish":
									cancel()
									return context.Canceled
								case "fatal cleanup":
									return ports.ErrPublisherUnusable
								}
								return nil
							})
						}
						switch scenario {
						case "success", "stale completion", "completion error":
							changed := scenario != "stale completion"
							var err error
							if scenario == "completion error" {
								err = errors.New("private-database")
							}
							repo.EXPECT().MarkPublished(gomock.Any(), event.ID, event.ClaimToken).Return(changed, err)
							wantError = scenario == "completion error"
							if scenario == "success" {
								wantDelay = 0
							}
						case "canceled publish":
							wantDelay = 0
						default:
							var err error
							if scenario == "retry error" {
								err = errors.New("private-database")
							}
							retryDelay := time.Second
							if malformed {
								retryDelay = app.OutboxMaxBackoff
								wantDelay = 0
							}
							repo.EXPECT().Retry(gomock.Any(), event.ID, event.ClaimToken, retryDelay).Return(scenario != "stale retry", err)
						}
					}
				}
			}
			var logs strings.Builder
			d := app.NewOutboxDispatcher(repo, publisher, slog.New(slog.NewJSONHandler(&logs, nil)))
			delay, err := d.Step(ctx)
			require.Equal(t, wantDelay, delay)
			if wantError {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-")
			} else {
				require.NoError(t, err)
			}
			require.NotContains(t, logs.String(), "private-")
		})
	}
}

func TestOutboxDispatcher_BackoffCapsAndResets(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	publisher := mocks.NewMockEventPublisher(ctrl)
	d := app.NewOutboxDispatcher(repo, publisher, nil)
	event := outgoing(t)
	for _, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second} {
		repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(event, true, nil)
		publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(errors.New("failure"))
		repo.EXPECT().Retry(gomock.Any(), event.ID, event.ClaimToken, delay).Return(true, nil)
		got, err := d.Step(context.Background())
		require.Error(t, err)
		require.Equal(t, delay, got)
	}
	repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(event, true, nil)
	publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().MarkPublished(gomock.Any(), event.ID, event.ClaimToken).Return(true, nil)
	_, err := d.Step(context.Background())
	require.NoError(t, err)
	repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(domain.OutgoingEvent{}, false, errors.New("failure"))
	got, err := d.Step(context.Background())
	require.Error(t, err)
	require.Equal(t, time.Second, got)
}

func TestOutboxDispatcher_RunCancelsWaitAndStopsOnFatalCleanup(t *testing.T) {
	for _, scenario := range []string{"idle", "failure", "fatal"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOutboxRepository(ctrl)
			publisher := mocks.NewMockEventPublisher(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			if scenario == "fatal" {
				event := outgoing(t)
				repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(event, true, nil)
				publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(ports.ErrPublisherUnusable)
				repo.EXPECT().Retry(gomock.Any(), event.ID, event.ClaimToken, time.Second).Return(true, nil)
			} else {
				repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).DoAndReturn(func(context.Context, time.Duration) (domain.OutgoingEvent, bool, error) {
					close(entered)
					if scenario == "failure" {
						return domain.OutgoingEvent{}, false, errors.New("private-error")
					}
					return domain.OutgoingEvent{}, false, nil
				})
			}
			done := make(chan error, 1)
			go func() {
				done <- app.NewOutboxDispatcher(repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(ctx)
			}()
			if scenario != "fatal" {
				<-entered
				cancel()
			}
			select {
			case err := <-done:
				if scenario == "fatal" {
					require.ErrorIs(t, err, ports.ErrPublisherUnusable)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(time.Second):
				t.Fatal("worker did not stop")
			}
		})
	}
}

func TestOutboxDispatcher_MalformedEventDoesNotStarveValidEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutboxRepository(ctrl)
	publisher := mocks.NewMockEventPublisher(ctrl)
	bad, good := outgoing(t), outgoing(t)
	bad.ID = "33333333-3333-4333-8333-333333333333"
	bad.Payload = "malformed saved envelope"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gomock.InOrder(
		repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(bad, true, nil),
		repo.EXPECT().Retry(gomock.Any(), bad.ID, bad.ClaimToken, app.OutboxMaxBackoff).Return(true, nil),
		repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(good, true, nil),
		publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *eventsv1.EventEnvelope) error {
			require.Equal(t, good.ID, e.EventId)
			return nil
		}),
		repo.EXPECT().MarkPublished(gomock.Any(), good.ID, good.ClaimToken).DoAndReturn(func(context.Context, string, string) (bool, error) { cancel(); return true, nil }),
	)
	done := make(chan error, 1)
	go func() { done <- app.NewOutboxDispatcher(repo, publisher, nil).Run(ctx) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("valid event blocked behind malformed event")
	}
}

func TestOutboxDispatcher_EventDeferralAndWorkerBackoffAreSeparate(t *testing.T) {
	for _, outcome := range []string{"scheduled", "stale", "database error"} {
		t.Run(outcome, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOutboxRepository(ctrl)
			publisher := mocks.NewMockEventPublisher(ctrl)
			bad := outgoing(t)
			bad.Payload = "malformed"
			repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(bad, true, nil)
			var retryErr error
			if outcome == "database error" {
				retryErr = errors.New("database failed")
			}
			repo.EXPECT().Retry(gomock.Any(), bad.ID, bad.ClaimToken, app.OutboxMaxBackoff).Return(outcome != "stale", retryErr)
			d := app.NewOutboxDispatcher(repo, publisher, nil)
			delay, err := d.Step(context.Background())
			require.Error(t, err)
			expected := time.Duration(0)
			if outcome == "stale" || outcome == "database error" {
				expected = time.Second
			}
			require.Equal(t, expected, delay)
			// No eligible events still requests the normal idle wait.
			repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(domain.OutgoingEvent{}, false, nil)
			delay, err = d.Step(context.Background())
			require.NoError(t, err)
			require.Equal(t, app.OutboxIdleWait, delay)
			// Event deferral does not advance or reset transient worker backoff.
			repo.EXPECT().Claim(gomock.Any(), app.OutboxLease).Return(domain.OutgoingEvent{}, false, errors.New("database failed"))
			delay, err = d.Step(context.Background())
			require.Error(t, err)
			expected = time.Second
			if outcome == "database error" {
				expected = 2 * time.Second
			}
			require.Equal(t, expected, delay)
		})
	}
}
