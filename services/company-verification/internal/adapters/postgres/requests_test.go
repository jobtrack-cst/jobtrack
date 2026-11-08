package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

// Internal test: the status mapping must round-trip and match the SQL CHECK values.
func TestStatusMapping_RoundTrips(t *testing.T) {
	for _, tt := range []struct {
		status domain.RequestStatus
		name   string
	}{
		{domain.StatusPending, "pending"},
		{domain.StatusApproved, "approved"},
		{domain.StatusRejected, "rejected"},
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
	if _, ok := statusText(domain.StatusUnspecified); ok {
		t.Error("statusText accepted StatusUnspecified")
	}
	for _, name := range []string{"", "Pending", "verified"} {
		if _, ok := parseStatus(name); ok {
			t.Errorf("parseStatus(%q) accepted an unknown value", name)
		}
	}
}

func TestUniqueError_MapsOnlyKnownIndexes(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want error
	}{
		{"one pending index", &pgconn.PgError{Code: uniqueViolation, ConstraintName: onePendingIndex}, domain.ErrPendingRequestExists},
		{"one approved index", &pgconn.PgError{Code: uniqueViolation, ConstraintName: oneApprovedIndex}, domain.ErrAlreadyVerified},
		{"primary key", &pgconn.PgError{Code: uniqueViolation, ConstraintName: "verification_requests_pkey"}, nil},
		{"other SQL error", &pgconn.PgError{Code: "23514", ConstraintName: onePendingIndex}, nil},
		{"not a Postgres error", errors.New("boom"), nil},
		{"no error", nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueError(tt.err); !errors.Is(got, tt.want) || (tt.want == nil && got != nil) {
				t.Fatalf("uniqueError()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestNullable(t *testing.T) {
	if got := nullable(""); got != nil {
		t.Errorf("nullable(\"\")=%v, want nil", got)
	}
	if got := nullable("abc"); got != "abc" {
		t.Errorf("nullable(\"abc\")=%v, want abc", got)
	}
}
