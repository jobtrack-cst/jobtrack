package domain_test

import (
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
)

func TestCaller_RequireApplicant(t *testing.T) {
	validUUID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name    string
		role    string
		wantErr error
	}{
		{"applicant allowed", "applicant", nil},
		{"employer denied", "employer", domain.ErrApplicantOnly},
		{"admin denied", "admin", domain.ErrApplicantOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := domain.NewCaller(validUUID, tt.role)
			if err != nil {
				t.Fatalf("NewCaller error: %v", err)
			}
			if err := c.RequireApplicant(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("RequireApplicant() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCaller_Invalid(t *testing.T) {
	if _, err := domain.NewCaller("not-a-uuid", "applicant"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
	if _, err := domain.NewCaller("11111111-1111-4111-8111-111111111111", ""); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
