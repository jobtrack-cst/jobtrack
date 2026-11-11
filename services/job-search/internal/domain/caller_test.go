package domain_test

import (
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

func TestNewCaller(t *testing.T) {
	valid := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name    string
		userID  string
		role    string
		want    domain.Caller
		wantErr error
	}{
		{"applicant", valid, "applicant", domain.Caller{UserID: valid, Role: "applicant"}, nil},
		{"employer", valid, "employer", domain.Caller{UserID: valid, Role: "employer"}, nil},
		{"admin", valid, "admin", domain.Caller{UserID: valid, Role: "admin"}, nil},
		{"empty id", "", "applicant", domain.Caller{}, domain.ErrUnauthenticated},
		{"bad uuid", "not-a-uuid", "applicant", domain.Caller{}, domain.ErrUnauthenticated},
		{"empty role", valid, "", domain.Caller{}, domain.ErrUnauthenticated},
		{"unknown role", valid, "superuser", domain.Caller{}, domain.ErrUnauthenticated},
		{"wrong case", valid, "Applicant", domain.Caller{}, domain.ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewCaller(tt.userID, tt.role)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
