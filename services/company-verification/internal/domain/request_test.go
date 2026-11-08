package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

func TestNewSubmission(t *testing.T) {
	// "ä" is 2 bytes, so these check that limits count characters, not bytes.
	reg50 := strings.Repeat("ä", 50)
	notes1000 := strings.Repeat("n", 1000)
	tests := []struct {
		name       string
		reg, notes string
		want       domain.Submission
		wantErr    error
	}{
		{name: "number and notes", reg: "BT-2026-001", notes: "Licensed in Thimphu",
			want: domain.Submission{RegistrationNumber: "BT-2026-001", Notes: "Licensed in Thimphu"}},
		{name: "only number", reg: "BT-1", want: domain.Submission{RegistrationNumber: "BT-1"}},
		{name: "trims both fields", reg: "  BT-1 ", notes: " note ", want: domain.Submission{RegistrationNumber: "BT-1", Notes: "note"}},
		// registration_number boundaries: 0, 1, 50, 51
		{name: "empty number", reg: "", wantErr: domain.ErrInvalidRegistrationNumber},
		{name: "spaces-only number", reg: "   ", wantErr: domain.ErrInvalidRegistrationNumber},
		{name: "1-character number", reg: "B", want: domain.Submission{RegistrationNumber: "B"}},
		{name: "50-character number", reg: reg50, want: domain.Submission{RegistrationNumber: reg50}},
		{name: "51-character number", reg: reg50 + "ä", wantErr: domain.ErrInvalidRegistrationNumber},
		{name: "number with NUL", reg: "BT\x001", wantErr: domain.ErrInvalidRegistrationNumber},
		// notes boundaries: 1000, 1001
		{name: "1000-character notes", reg: "B", notes: notes1000, want: domain.Submission{RegistrationNumber: "B", Notes: notes1000}},
		{name: "1001-character notes", reg: "B", notes: notes1000 + "n", wantErr: domain.ErrInvalidNotes},
		{name: "notes with NUL", reg: "B", notes: "a\x00b", wantErr: domain.ErrInvalidNotes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewSubmission(tt.reg, tt.notes)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got %+v,%v, want %+v,%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestNewRejectionReason(t *testing.T) {
	reason500 := strings.Repeat("ä", 500)
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"normal reason", "Registration number not found", "Registration number not found", nil},
		{"trimmed", "  Expired licence ", "Expired licence", nil},
		// boundaries: 0, 1, 500, 501
		{"empty", "", "", domain.ErrInvalidRejectionReason},
		{"spaces only", "   ", "", domain.ErrInvalidRejectionReason},
		{"1 character", "x", "x", nil},
		{"500 characters", reason500, reason500, nil},
		{"501 characters", reason500 + "ä", "", domain.ErrInvalidRejectionReason},
		{"contains NUL", "bad\x00reason", "", domain.ErrInvalidRejectionReason},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewRejectionReason(tt.in)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got %q,%v, want %q,%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestCheckCanSubmit(t *testing.T) {
	for _, tt := range []struct {
		name     string
		existing []domain.RequestStatus
		wantErr  error
	}{
		{"first request", nil, nil},
		{"after one rejection", []domain.RequestStatus{domain.StatusRejected}, nil},
		{"after several rejections", []domain.RequestStatus{domain.StatusRejected, domain.StatusRejected}, nil},
		{"pending exists", []domain.RequestStatus{domain.StatusPending}, domain.ErrPendingRequestExists},
		{"pending after a rejection", []domain.RequestStatus{domain.StatusRejected, domain.StatusPending}, domain.ErrPendingRequestExists},
		{"already approved", []domain.RequestStatus{domain.StatusApproved}, domain.ErrAlreadyVerified},
		{"approved after a rejection", []domain.RequestStatus{domain.StatusRejected, domain.StatusApproved}, domain.ErrAlreadyVerified},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.CheckCanSubmit(tt.existing); !errors.Is(err, tt.wantErr) {
				t.Fatalf("CheckCanSubmit()=%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequestStatus_ValidFilter(t *testing.T) {
	for _, tt := range []struct {
		status domain.RequestStatus
		want   bool
	}{
		{domain.StatusUnspecified, true},
		{domain.StatusPending, true},
		{domain.StatusApproved, true},
		{domain.StatusRejected, true},
		{domain.StatusRejected + 1, false},
		{99, false},
	} {
		if got := tt.status.ValidFilter(); got != tt.want {
			t.Errorf("ValidFilter(%d)=%v, want %v", tt.status, got, tt.want)
		}
	}
}

var btt = time.FixedZone("BTT", 6*3600)

func newPending(t *testing.T) domain.VerificationRequest {
	t.Helper()
	s := domain.Submission{RegistrationNumber: "BT-1", Notes: "note"}
	return domain.NewVerificationRequest(requestID, employerID, s, time.Date(2026, 11, 8, 10, 0, 0, 0, btt))
}

func TestNewVerificationRequest_StartsPendingInUTC(t *testing.T) {
	r := newPending(t)
	if r.Status != domain.StatusPending {
		t.Errorf("status=%v, want StatusPending", r.Status)
	}
	if r.ID != requestID || r.EmployerID != employerID || r.RegistrationNumber != "BT-1" || r.Notes != "note" {
		t.Errorf("fields not copied: %+v", r)
	}
	if r.ReviewedBy != "" || r.RejectionReason != "" {
		t.Errorf("reviewer=%q reason=%q, want both empty", r.ReviewedBy, r.RejectionReason)
	}
	if r.CreatedAt.Location() != time.UTC || !r.UpdatedAt.Equal(r.CreatedAt) {
		t.Errorf("timestamps=%v,%v, want equal and in UTC", r.CreatedAt, r.UpdatedAt)
	}
}

// State transitions: only PENDING can move, to APPROVED or REJECTED, once.
func TestVerificationRequest_Decisions(t *testing.T) {
	later := time.Date(2026, 11, 8, 12, 0, 0, 0, btt)
	approve := func(r *domain.VerificationRequest) error { return r.Approve(adminID, later) }
	reject := func(r *domain.VerificationRequest) error { return r.Reject(adminID, "Expired licence", later) }
	for _, tt := range []struct {
		name       string
		from       domain.RequestStatus
		decide     func(*domain.VerificationRequest) error
		wantErr    error
		wantStatus domain.RequestStatus
		wantReason string
	}{
		{"approve pending", domain.StatusPending, approve, nil, domain.StatusApproved, ""},
		{"reject pending", domain.StatusPending, reject, nil, domain.StatusRejected, "Expired licence"},
		{"approve approved", domain.StatusApproved, approve, domain.ErrRequestNotPending, domain.StatusApproved, ""},
		{"approve rejected", domain.StatusRejected, approve, domain.ErrRequestNotPending, domain.StatusRejected, ""},
		{"reject approved", domain.StatusApproved, reject, domain.ErrRequestNotPending, domain.StatusApproved, ""},
		{"reject rejected", domain.StatusRejected, reject, domain.ErrRequestNotPending, domain.StatusRejected, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newPending(t)
			r.Status = tt.from
			before := r
			err := tt.decide(&r)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if r != before {
					t.Fatalf("request changed on a failed decision: %+v", r)
				}
				return
			}
			if r.Status != tt.wantStatus || r.ReviewedBy != adminID || r.RejectionReason != tt.wantReason {
				t.Errorf("got status=%v reviewer=%q reason=%q", r.Status, r.ReviewedBy, r.RejectionReason)
			}
			if !r.UpdatedAt.Equal(later) || r.UpdatedAt.Location() != time.UTC || !r.CreatedAt.Equal(before.CreatedAt) {
				t.Errorf("timestamps=%v,%v", r.CreatedAt, r.UpdatedAt)
			}
		})
	}
}
