package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

// stubVerifications satisfies the port in wiring tests that never call it.
type stubVerifications struct{ ports.VerificationRepository }

func TestRunWithDatabase_WiresVerificationRepository(t *testing.T) {
	cfg := config{databaseURL: "database-config", migrationsDir: t.TempDir()}
	started := false
	err := runWithDatabase(context.Background(), cfg, logging.New("test", io.Discard),
		func(context.Context, string) (*databaseHandle, error) {
			return &databaseHandle{
				readiness:     readinessFunc(func(context.Context) error { return nil }),
				profiles:      stubProfiles{},
				verifications: stubVerifications{},
				migrate:       func(context.Context, string) error { return nil },
				close:         func() {},
			}, nil
		}, func(_ context.Context, _ config, _ *slog.Logger, d deps) error {
			started = true
			if d.verifications == nil {
				t.Error("verification repository was not wired")
			}
			return nil
		})
	if err != nil || !started {
		t.Fatalf("err=%v started=%v", err, started)
	}
}
