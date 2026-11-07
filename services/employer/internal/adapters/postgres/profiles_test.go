package postgres

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// Internal test: the status mapping must round-trip and match the SQL CHECK values.
func TestStatusMapping_RoundTrips(t *testing.T) {
	for _, tt := range []struct {
		status domain.VerificationStatus
		name   string
	}{
		{domain.VerificationUnverified, "unverified"},
		{domain.VerificationVerified, "verified"},
		{domain.VerificationRejected, "rejected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := statusText(tt.status)
			if !ok || got != tt.name {
				t.Fatalf("statusText(%v)=%q,%v, want %q,true", tt.status, got, ok, tt.name)
			}
			back, ok := parseStatus(tt.name)
			if !ok || back != tt.status {
				t.Fatalf("parseStatus(%q)=%v,%v, want %v,true", tt.name, back, ok, tt.status)
			}
		})
	}
}

func TestStatusMapping_RejectsUnknownValues(t *testing.T) {
	if _, ok := statusText(domain.VerificationUnspecified); ok {
		t.Error("statusText accepted VerificationUnspecified")
	}
	for _, name := range []string{"", "Verified", "pending"} {
		if _, ok := parseStatus(name); ok {
			t.Errorf("parseStatus(%q) accepted an unknown value", name)
		}
	}
}
