package domain_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pdf returns n bytes that start with the PDF signature (n must be at least 5).
func pdf(n int) []byte {
	return append([]byte("%PDF-"), bytes.Repeat([]byte("x"), n-5)...)
}

func TestNewUpload(t *testing.T) {
	valid := domain.UploadInput{FileName: "cv.pdf", ContentType: "application/pdf", Content: pdf(100)}
	with := func(change func(*domain.UploadInput)) domain.UploadInput {
		in := valid
		change(&in)
		return in
	}
	tests := []struct {
		name     string
		in       domain.UploadInput
		wantName string
		wantErr  error
	}{
		{"valid", valid, "cv.pdf", nil},

		// Content size boundaries around 2,000,000 bytes (AGENTS.md Section 10).
		{"size 1,999,999 bytes", with(func(in *domain.UploadInput) { in.Content = pdf(1_999_999) }), "cv.pdf", nil},
		{"size 2,000,000 bytes", with(func(in *domain.UploadInput) { in.Content = pdf(2_000_000) }), "cv.pdf", nil},
		{"size 2,000,001 bytes", with(func(in *domain.UploadInput) { in.Content = pdf(2_000_001) }), "", domain.ErrInvalidContentSize},
		{"empty content", with(func(in *domain.UploadInput) { in.Content = nil }), "", domain.ErrInvalidContentSize},
		{"smallest pdf, signature only", with(func(in *domain.UploadInput) { in.Content = []byte("%PDF-") }), "cv.pdf", nil},

		// PDF signature.
		{"not a pdf", with(func(in *domain.UploadInput) { in.Content = []byte("hello world") }), "", domain.ErrInvalidPDF},
		{"signature not at start", with(func(in *domain.UploadInput) { in.Content = []byte(" %PDF-1.7") }), "", domain.ErrInvalidPDF},
		{"lowercase signature", with(func(in *domain.UploadInput) { in.Content = []byte("%pdf-1.7") }), "", domain.ErrInvalidPDF},

		// Content type.
		{"empty content type", with(func(in *domain.UploadInput) { in.ContentType = "" }), "", domain.ErrInvalidContentType},
		{"wrong content type", with(func(in *domain.UploadInput) { in.ContentType = "text/plain" }), "", domain.ErrInvalidContentType},
		{"content type wrong case", with(func(in *domain.UploadInput) { in.ContentType = "Application/PDF" }), "", domain.ErrInvalidContentType},

		// File name length boundaries, in characters after trimming.
		{"name 1 character", with(func(in *domain.UploadInput) { in.FileName = "a" }), "a", nil},
		{"name 255 characters", with(func(in *domain.UploadInput) { in.FileName = strings.Repeat("a", 255) }), strings.Repeat("a", 255), nil},
		{"name 256 characters", with(func(in *domain.UploadInput) { in.FileName = strings.Repeat("a", 256) }), "", domain.ErrInvalidFileName},
		{"name 255 multibyte characters", with(func(in *domain.UploadInput) { in.FileName = strings.Repeat("é", 255) }), strings.Repeat("é", 255), nil},
		{"name trimmed", with(func(in *domain.UploadInput) { in.FileName = "  cv.pdf  " }), "cv.pdf", nil},
		{"name 255 after trimming", with(func(in *domain.UploadInput) { in.FileName = " " + strings.Repeat("a", 255) + " " }), strings.Repeat("a", 255), nil},
		{"empty name", with(func(in *domain.UploadInput) { in.FileName = "" }), "", domain.ErrInvalidFileName},
		{"whitespace name", with(func(in *domain.UploadInput) { in.FileName = "   " }), "", domain.ErrInvalidFileName},
		{"name with slash", with(func(in *domain.UploadInput) { in.FileName = "../cv.pdf" }), "", domain.ErrInvalidFileName},
		{"name with backslash", with(func(in *domain.UploadInput) { in.FileName = `docs\cv.pdf` }), "", domain.ErrInvalidFileName},
		{"name with NUL", with(func(in *domain.UploadInput) { in.FileName = "cv\x00.pdf" }), "", domain.ErrInvalidFileName},
		{"name invalid UTF-8", with(func(in *domain.UploadInput) { in.FileName = "cv\xff.pdf" }), "", domain.ErrInvalidFileName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewUpload(tt.in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got.FileName)
			assert.Equal(t, "application/pdf", got.ContentType)
			assert.Equal(t, tt.in.Content, got.Content)
		})
	}
}

func TestResume_CheckOwner(t *testing.T) {
	r := domain.Resume{ID: otherID, ApplicantID: applicantID}
	tests := []struct {
		name    string
		caller  domain.Caller
		wantErr error
	}{
		{"owner", domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}, nil},
		{"other applicant", domain.Caller{UserID: otherID, Role: domain.RoleApplicant}, domain.ErrNotResumeOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.CheckOwner(tt.caller)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidationStatus_RoundTrip(t *testing.T) {
	tests := []struct {
		status domain.ValidationStatus
		stored string
	}{
		{domain.ValidationStatusPending, "pending"},
		{domain.ValidationStatusValidated, "validated"},
		{domain.ValidationStatusRejected, "rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.stored, func(t *testing.T) {
			assert.Equal(t, tt.stored, tt.status.String())
			got, ok := domain.ParseValidationStatus(tt.stored)
			require.True(t, ok)
			assert.Equal(t, tt.status, got)
		})
	}
	for _, bad := range []string{"", "PENDING", "unknown"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			got, ok := domain.ParseValidationStatus(bad)
			assert.False(t, ok)
			assert.Equal(t, domain.ValidationStatusUnspecified, got)
		})
	}
	assert.Equal(t, "", domain.ValidationStatusUnspecified.String())
}
