package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/stretchr/testify/require"
)

func TestStartup_TopologyAfterMigrationBeforePublisherAndListeners(t *testing.T) {
	for _, failure := range []string{"", "migrate", "topology"} {
		t.Run(failure, func(t *testing.T) {
			var calls []string
			cfg := config{databaseURL: "synthetic-db", migrationsDir: "synthetic-migrations", rabbitMQURL: "synthetic-broker"}
			err := runWithDatabase(context.Background(), cfg, logging.New("resume", io.Discard), func(context.Context, string) (*databaseHandle, error) {
				return &databaseHandle{migrate: func(context.Context, string) error {
					calls = append(calls, "migrate")
					if failure == "migrate" {
						return errors.New("migration failed")
					}
					return nil
				}, close: func() { calls = append(calls, "close database") }}, nil
			}, func(ctx context.Context, cfg config, logger *slog.Logger, d deps) error {
				return runWithTopology(ctx, cfg, logger, d, func(ctx context.Context, broker string) error {
					require.Equal(t, cfg.rabbitMQURL, broker)
					calls = append(calls, "topology")
					if failure == "topology" {
						return errors.New("topology incompatible; reviewed migration required")
					}
					return nil
				}, func(context.Context, config, *slog.Logger, deps) error {
					calls = append(calls, "publisher and listeners")
					return nil
				})
			})
			expected := []string{"migrate", "topology", "publisher and listeners", "close database"}
			if failure == "migrate" {
				expected = []string{"migrate", "close database"}
			} else if failure == "topology" {
				expected = []string{"migrate", "topology", "close database"}
			}
			require.Equal(t, expected, calls)
			if failure == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
