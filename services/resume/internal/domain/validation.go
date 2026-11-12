package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidValidationResult     = errors.New("invalid validation result")
	ErrValidationApplicantMismatch = errors.New("validation applicant does not match resume")
)

const MaxRejectionReasonLength = 500

// ValidationResult is transport-independent. Rejection reasons are trimmed.
type ValidationResult struct {
	EventID     string
	ResumeID    string
	ApplicantID string
	Status      ValidationStatus
	Reason      string
}

type ValidationOutcome uint8

const (
	ValidationOutcomeUnspecified ValidationOutcome = iota
	ValidationApplied
	ValidationDuplicate
	ValidationAlreadyFinal
)

func NewValidationResult(eventID, resumeID, applicantID string, status ValidationStatus, reason string) (ValidationResult, error) {
	event, ok := normalizeUUID(eventID)
	if !ok {
		return ValidationResult{}, ErrInvalidValidationResult
	}
	resume, ok := normalizeUUID(resumeID)
	if !ok {
		return ValidationResult{}, ErrInvalidValidationResult
	}
	applicant, ok := normalizeUUID(applicantID)
	if !ok {
		return ValidationResult{}, ErrInvalidValidationResult
	}
	if status != ValidationStatusValidated && status != ValidationStatusRejected {
		return ValidationResult{}, ErrInvalidValidationResult
	}
	// UTF-8 and NUL checks are storage/protobuf requirements, not a text-length policy.
	if !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) || (status == ValidationStatusValidated && reason != "") {
		return ValidationResult{}, ErrInvalidValidationResult
	}
	if status == ValidationStatusRejected {
		reason = strings.TrimSpace(reason)
		if n := utf8.RuneCountInString(reason); n < 1 || n > MaxRejectionReasonLength {
			return ValidationResult{}, ErrInvalidValidationResult
		}
	}
	return ValidationResult{EventID: event, ResumeID: resume, ApplicantID: applicant, Status: status, Reason: reason}, nil
}

// AcceptsValidationResult enforces first-final-result wins, including conflicts.
func (s ValidationStatus) AcceptsValidationResult(target ValidationStatus) (bool, error) {
	if target != ValidationStatusValidated && target != ValidationStatusRejected {
		return false, ErrInvalidValidationResult
	}
	switch s {
	case ValidationStatusPending:
		return true, nil
	case ValidationStatusValidated, ValidationStatusRejected:
		return false, nil
	default:
		return false, ErrInvalidValidationResult
	}
}
