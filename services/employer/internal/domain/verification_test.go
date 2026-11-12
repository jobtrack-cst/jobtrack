package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

const verificationEventID = "8a1f0c52-3d4e-4b6a-9c7d-1e2f3a4b5c6d"

func TestNewVerificationDecision(t *testing.T) {
	decidedAt := time.Date(2026, 11, 12, 10, 30, 0, 0, time.FixedZone("BTT", 6*3600))
	tests := []struct {
		name      string
		eventID   string
		employer  string
		status    domain.VerificationStatus
		reason    string
		decidedAt time.Time
		valid     bool
	}{
		{"approval", verificationEventID, employerID, domain.VerificationVerified, "", decidedAt, true},
		{"rejection", verificationEventID, employerID, domain.VerificationRejected, "Expired licence", decidedAt, true},
		{"reason of 1 character", verificationEventID, employerID, domain.VerificationRejected, "x", decidedAt, true},
		{"reason of 500 characters", verificationEventID, employerID, domain.VerificationRejected, strings.Repeat("é", 500), decidedAt, true},
		{"reason of 501 characters", verificationEventID, employerID, domain.VerificationRejected, strings.Repeat("é", 501), decidedAt, false},
		{"rejection without reason", verificationEventID, employerID, domain.VerificationRejected, "   ", decidedAt, false},
		{"reason with NUL", verificationEventID, employerID, domain.VerificationRejected, "bad\x00reason", decidedAt, false},
		{"reason with invalid UTF-8", verificationEventID, employerID, domain.VerificationRejected, "bad\xffreason", decidedAt, false},
		{"approval with reason", verificationEventID, employerID, domain.VerificationVerified, "why", decidedAt, false},
		{"unverified is not a decision", verificationEventID, employerID, domain.VerificationUnverified, "", decidedAt, false},
		{"unspecified status", verificationEventID, employerID, domain.VerificationUnspecified, "", decidedAt, false},
		{"event ID not a UUID", "event-1", employerID, domain.VerificationVerified, "", decidedAt, false},
		{"employer ID not a UUID", verificationEventID, "employer-1", domain.VerificationVerified, "", decidedAt, false},
		{"missing decision time", verificationEventID, employerID, domain.VerificationVerified, "", time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewVerificationDecision(tt.eventID, tt.employer, tt.status, tt.reason, tt.decidedAt)
			if !tt.valid {
				require.ErrorIs(t, err, domain.ErrInvalidVerificationEvent)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.status, got.Status)
			require.Equal(t, strings.TrimSpace(tt.reason), got.RejectionReason)
			require.Equal(t, time.UTC, got.DecidedAt.Location(), "stored in UTC")
			require.True(t, got.DecidedAt.Equal(tt.decidedAt))
		})
	}
}

func TestNewVerificationDecision_NormalizesIDsAndTrimsReason(t *testing.T) {
	got, err := domain.NewVerificationDecision(strings.ToUpper(verificationEventID), strings.ToUpper(employerID),
		domain.VerificationRejected, "  Expired licence ", time.Now())
	require.NoError(t, err)
	require.Equal(t, verificationEventID, got.EventID)
	require.Equal(t, employerID, got.EmployerID)
	require.Equal(t, "Expired licence", got.RejectionReason)
}

func TestVerificationOutcome_String(t *testing.T) {
	for outcome, want := range map[domain.VerificationOutcome]string{
		domain.OutcomeApplied:              "applied",
		domain.OutcomeStoredWithoutProfile: "stored_without_profile",
		domain.OutcomeDuplicate:            "duplicate",
		domain.OutcomeStale:                "stale",
		domain.VerificationOutcome(0):      "unknown",
	} {
		require.Equal(t, want, outcome.String())
	}
}
