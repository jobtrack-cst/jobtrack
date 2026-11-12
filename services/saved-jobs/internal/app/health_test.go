package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/ports/mocks"
)

func TestHealth_CheckHealth(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		want domain.HealthStatus
		err  error
	}{
		{"healthy", context.Background(), domain.Serving, nil},
		{"canceled", canceled, domain.Unknown, context.Canceled},
		{"expired", expired, domain.Unknown, context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.NewHealth().CheckHealth(tt.ctx)
			if got != tt.want || !errors.Is(err, tt.err) {
				t.Fatalf("got %v, %v; want %v, %v", got, err, tt.want, tt.err)
			}
		})
	}
}

func TestHealth_CheckHealth_UsesReadinessPort(t *testing.T) {
	dependencyErr := errors.New("dependency unavailable")
	for _, tt := range []struct {
		name string
		err  error
		want domain.HealthStatus
	}{
		{"ready", nil, domain.Serving},
		{"not ready", dependencyErr, domain.NotServing},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			readiness := mocks.NewMockReadiness(gomock.NewController(t))
			// Times(1) fails the test if the port is not called exactly once with this context.
			readiness.EXPECT().Check(ctx).Return(tt.err).Times(1)
			got, err := app.NewHealthWithReadiness(readiness).CheckHealth(ctx)
			if got != tt.want || !errors.Is(err, tt.err) {
				t.Fatalf("status=%v err=%v; want %v, %v", got, err, tt.want, tt.err)
			}
		})
	}
}
