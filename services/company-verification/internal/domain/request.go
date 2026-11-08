package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits in characters, after trimming spaces. Noted in the JT-33 PR
// and used for boundary value tests.
const (
	MaxRegistrationNumberLength = 50
	MaxNotesLength              = 1000
	MaxRejectionReasonLength    = 500
)

// RequestStatus values match jobtrack.company_verification.v1.RequestStatus.
type RequestStatus uint8

const (
	StatusUnspecified RequestStatus = iota
	StatusPending
	StatusApproved
	StatusRejected
)

// ValidFilter reports whether s can filter ListVerificationRequests.
// StatusUnspecified means all statuses.
func (s RequestStatus) ValidFilter() bool {
	return s <= StatusRejected
}

// VerificationRequest is one employer's request to be verified.
type VerificationRequest struct {
	ID                 string
	EmployerID         string
	RegistrationNumber string
	Notes              string
	Status             RequestStatus
	RejectionReason    string
	ReviewedBy         string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Submission is the validated, trimmed input of SubmitVerificationRequest.
type Submission struct {
	RegistrationNumber string
	Notes              string
}

// NewSubmission validates the employer's input.
func NewSubmission(registrationNumber, notes string) (Submission, error) {
	s := Submission{
		RegistrationNumber: strings.TrimSpace(registrationNumber),
		Notes:              strings.TrimSpace(notes),
	}
	if !textWithin(s.RegistrationNumber, 1, MaxRegistrationNumberLength) {
		return Submission{}, ErrInvalidRegistrationNumber
	}
	if !textWithin(s.Notes, 0, MaxNotesLength) {
		return Submission{}, ErrInvalidNotes
	}
	return s, nil
}

// NewRejectionReason validates the admin's reason. Empty is INVALID_ARGUMENT
// as the proto says.
func NewRejectionReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if !textWithin(reason, 1, MaxRejectionReasonLength) {
		return "", ErrInvalidRejectionReason
	}
	return reason, nil
}

// textWithin counts characters, not bytes. NUL is refused because Postgres
// text columns cannot store it, which would surface as INTERNAL.
func textWithin(s string, minLen, maxLen int) bool {
	if strings.ContainsRune(s, 0) || !utf8.ValidString(s) {
		return false
	}
	n := utf8.RuneCountInString(s)
	return n >= minLen && n <= maxLen
}

// CheckCanSubmit applies the proto rules to the employer's existing requests:
// approved before is FAILED_PRECONDITION, a pending one is ALREADY_EXISTS,
// and after only rejections a new request is allowed.
func CheckCanSubmit(existing []RequestStatus) error {
	pending := false
	for _, s := range existing {
		switch s {
		case StatusApproved:
			return ErrAlreadyVerified
		case StatusPending:
			pending = true
		}
	}
	if pending {
		return ErrPendingRequestExists
	}
	return nil
}

// NewVerificationRequest starts every request as PENDING with no reviewer.
func NewVerificationRequest(id, employerID string, s Submission, now time.Time) VerificationRequest {
	now = now.UTC()
	return VerificationRequest{
		ID:                 id,
		EmployerID:         employerID,
		RegistrationNumber: s.RegistrationNumber,
		Notes:              s.Notes,
		Status:             StatusPending,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// Approve decides a PENDING request. Any other status is FAILED_PRECONDITION.
func (r *VerificationRequest) Approve(adminID string, now time.Time) error {
	return r.decide(StatusApproved, adminID, "", now)
}

// Reject decides a PENDING request with a reason already checked by
// NewRejectionReason.
func (r *VerificationRequest) Reject(adminID, reason string, now time.Time) error {
	return r.decide(StatusRejected, adminID, reason, now)
}

func (r *VerificationRequest) decide(next RequestStatus, adminID, reason string, now time.Time) error {
	if r.Status != StatusPending {
		return ErrRequestNotPending
	}
	r.Status = next
	r.ReviewedBy = adminID
	r.RejectionReason = reason
	r.UpdatedAt = now.UTC()
	return nil
}
