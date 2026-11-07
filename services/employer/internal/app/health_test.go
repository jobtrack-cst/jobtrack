package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
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

type readinessFunc func(context.Context) error

func (f readinessFunc) Check(ctx context.Context) error { return f(ctx) }

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
			calls := 0
			health := app.NewHealthWithReadiness(readinessFunc(func(got context.Context) error {
				calls++
				if got != ctx {
					t.Error("readiness context changed")
				}
				return tt.err
			}))
			got, err := health.CheckHealth(ctx)
			if calls != 1 || got != tt.want || !errors.Is(err, tt.err) {
				t.Fatalf("calls=%d status=%v err=%v", calls, got, err)
			}
		})
	}
}
