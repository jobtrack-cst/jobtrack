package domain_test

import (
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestOutbox_DurationsRespectDatabasePrecision(t *testing.T) {
	for _, tc := range []struct {
		name             string
		duration         time.Duration
		leaseOK, retryOK bool
	}{
		{"negative", -time.Microsecond, false, false},
		{"zero", 0, false, true},
		{"submicrosecond", time.Nanosecond, false, false},
		{"microsecond", time.Microsecond, true, true},
		{"fractional", time.Microsecond + time.Nanosecond, false, false},
		{"minute", time.Minute, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.leaseOK {
				require.NoError(t, domain.ValidateOutboxLease(tc.duration))
			} else {
				require.ErrorIs(t, domain.ValidateOutboxLease(tc.duration), domain.ErrInvalidOutboxLease)
			}
			if tc.retryOK {
				require.NoError(t, domain.ValidateOutboxRetryDelay(tc.duration))
			} else {
				require.ErrorIs(t, domain.ValidateOutboxRetryDelay(tc.duration), domain.ErrInvalidOutboxRetryDelay)
			}
		})
	}
}

func TestOutbox_ClaimIdentity(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name, id, token string
		valid           bool
	}{
		{"valid", id, id, true}, {"empty ID", "", id, false}, {"bad ID", "bad", id, false},
		{"empty token", id, "", false}, {"bad token", id, "bad", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateOutboxClaim(tc.id, tc.token)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, domain.ErrInvalidOutboxClaim)
			}
		})
	}
}
