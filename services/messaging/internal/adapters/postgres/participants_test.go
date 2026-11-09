package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
)

func TestParticipants_GetByApplicationID_RejectsInvalidID(t *testing.T) {
	for _, id := range []string{"", "not-a-uuid", "11111111111141118111111111111111", " 11111111-1111-4111-8111-111111111111", "gggggggg-1111-4111-8111-111111111111"} {
		t.Run(id, func(t *testing.T) {
			got, err := postgres.NewParticipants(nil).GetByApplicationID(context.Background(), id)
			if !errors.Is(err, domain.ErrInvalidApplicationID) || got != (domain.ApplicationParticipants{}) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestParticipants_GetByApplicationID_Unavailable(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing pool", true: "canceled"}[canceled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			got, err := postgres.NewParticipants(nil).GetByApplicationID(ctx, "11111111-1111-4111-8111-111111111111")
			if err == nil || errors.Is(err, domain.ErrParticipantsNotFound) || errors.Is(err, domain.ErrInvalidApplicationID) || got != (domain.ApplicationParticipants{}) {
				t.Fatalf("got %+v, %v", got, err)
			}
			if canceled && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}
