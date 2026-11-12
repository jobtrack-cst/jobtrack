package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestOutbox_InvalidArgumentsNeverAccessDatabase(t *testing.T) {
	repo := postgres.NewOutbox(nil)
	const id = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name string
		call func() error
		want error
	}{
		{"invalid lease", func() error { _, _, err := repo.Claim(context.Background(), 0); return err }, domain.ErrInvalidOutboxLease},
		{"invalid completion", func() error { _, err := repo.MarkPublished(context.Background(), id, ""); return err }, domain.ErrInvalidOutboxClaim},
		{"invalid retry token", func() error { _, err := repo.Retry(context.Background(), id, "bad", 0); return err }, domain.ErrInvalidOutboxClaim},
		{"invalid retry delay", func() error { _, err := repo.Retry(context.Background(), id, id, -time.Second); return err }, domain.ErrInvalidOutboxRetryDelay},
	} {
		t.Run(tc.name, func(t *testing.T) { require.ErrorIs(t, tc.call(), tc.want) })
	}
}
