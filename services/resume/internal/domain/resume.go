package domain

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"
)

// Upload limits from the resume.proto contract. The migration enforces the
// same values, so update both together.
const (
	MaxFileNameLength = 255
	MaxContentBytes   = 2_000_000
	PDFContentType    = "application/pdf"
)

// pdfSignature starts every PDF file. Deeper checks belong to resume-validator.
var pdfSignature = []byte("%PDF-")

// ValidationStatus starts as PENDING. VALIDATED and REJECTED are set only by
// the resume.validated and resume.rejected consumers (Sprint 2).
type ValidationStatus uint8

const (
	ValidationStatusUnspecified ValidationStatus = iota
	ValidationStatusPending
	ValidationStatusValidated
	ValidationStatusRejected
)

// String returns the value stored in the database validation_status column.
func (s ValidationStatus) String() string {
	switch s {
	case ValidationStatusPending:
		return "pending"
	case ValidationStatusValidated:
		return "validated"
	case ValidationStatusRejected:
		return "rejected"
	default:
		return ""
	}
}

// ParseValidationStatus reads a stored validation_status value.
func ParseValidationStatus(s string) (ValidationStatus, bool) {
	for _, st := range []ValidationStatus{ValidationStatusPending, ValidationStatusValidated, ValidationStatusRejected} {
		if st.String() == s {
			return st, true
		}
	}
	return ValidationStatusUnspecified, false
}

// Resume is the stored metadata. All IDs are lowercase UUID strings.
type Resume struct {
	ID               string
	ApplicantID      string
	FileName         string
	ContentType      string
	SizeBytes        int64
	ValidationStatus ValidationStatus
	RejectionReason  string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CheckOwner allows access only to the applicant who uploaded the resume.
func (r Resume) CheckOwner(c Caller) error {
	if r.ApplicantID != c.UserID {
		return ErrNotResumeOwner
	}
	return nil
}

// ResumeFile is a resume with its stored content, for downloads and GetResumeFile.
type ResumeFile struct {
	Resume
	Content []byte
}

// UploadInput is the raw content of an UploadResume request.
type UploadInput struct {
	FileName    string
	ContentType string
	Content     []byte
}

// Upload is validated, normalized upload content.
type Upload struct {
	FileName    string
	ContentType string
	Content     []byte
}

// NewUpload applies the contract rules: a file name, MIME type application/pdf,
// 1 to MaxContentBytes bytes, and content that starts with the PDF signature.
func NewUpload(in UploadInput) (Upload, error) {
	name, ok := cleanFileName(in.FileName)
	if !ok {
		return Upload{}, ErrInvalidFileName
	}
	if in.ContentType != PDFContentType {
		return Upload{}, ErrInvalidContentType
	}
	if len(in.Content) == 0 || len(in.Content) > MaxContentBytes {
		return Upload{}, ErrInvalidContentSize
	}
	if !bytes.HasPrefix(in.Content, pdfSignature) {
		return Upload{}, ErrInvalidPDF
	}
	return Upload{FileName: name, ContentType: in.ContentType, Content: in.Content}, nil
}

// cleanFileName trims s and refuses invalid UTF-8, NUL and path separators,
// so a stored name can never be read as a path.
func cleanFileName(s string) (string, bool) {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) || strings.ContainsAny(s, `/\`) {
		return "", false
	}
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= MaxFileNameLength
}
