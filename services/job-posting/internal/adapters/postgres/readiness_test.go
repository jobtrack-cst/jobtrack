package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/postgres"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestReadiness_Check(t *testing.T) {
	for _, name := range []string{"ready", "failure", "canceled", "missing pool"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "canceled" {
				cancel()
			}
			calls := 0
			var pool postgres.Pinger = pingerFunc(func(got context.Context) error {
				calls++
				if got != ctx {
					t.Error("caller context changed")
				}
				if name == "failure" {
					return errors.New("private credentials")
				}
				return nil
			})
			if name == "missing pool" {
				pool = nil
			}
			err := postgres.NewReadiness(pool).Check(ctx)
			if (err == nil) != (name == "ready") {
				t.Fatalf("unexpected error %v", err)
			}
			wantCalls := 1
			if name == "canceled" || name == "missing pool" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d want %d", calls, wantCalls)
			}
			if name == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("database details leaked")
			}
		})
	}
}
