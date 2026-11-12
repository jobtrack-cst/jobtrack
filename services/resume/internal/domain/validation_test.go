package domain_test

import (
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidationStatus_FirstFinalResultWins(t *testing.T) {
	for _, current := range []domain.ValidationStatus{domain.ValidationStatusUnspecified, domain.ValidationStatusPending, domain.ValidationStatusValidated, domain.ValidationStatusRejected} {
		for _, target := range []domain.ValidationStatus{domain.ValidationStatusPending, domain.ValidationStatusValidated, domain.ValidationStatusRejected} {
			t.Run(current.String()+"->"+target.String(), func(t *testing.T) {
				apply, err := current.AcceptsValidationResult(target)
				if current == domain.ValidationStatusUnspecified || target == domain.ValidationStatusPending {
					require.ErrorIs(t, err, domain.ErrInvalidValidationResult)
					require.False(t, apply)
				} else {
					require.NoError(t, err)
					require.Equal(t, current == domain.ValidationStatusPending, apply)
				}
			})
		}
	}
}

func TestNewValidationResult_Input(t *testing.T) {
	const id = "AAAAAAAA-1111-4222-8333-444455556666"
	for _, tc := range []struct {
		name                     string
		event, resume, applicant string
		status                   domain.ValidationStatus
		reason                   string
		valid                    bool
	}{
		{"validated", id, id, id, domain.ValidationStatusValidated, "", true},
		{"reason 1", id, id, id, domain.ValidationStatusRejected, "x", true},
		{"reason 500 unicode", id, id, id, domain.ValidationStatusRejected, strings.Repeat("ཀ", 500), true},
		{"reason 501 unicode", id, id, id, domain.ValidationStatusRejected, strings.Repeat("ཀ", 501), false},
		{"whitespace", id, id, id, domain.ValidationStatusRejected, " \t ", false},
		{"rejected", id, id, id, domain.ValidationStatusRejected, " reason as sent ", true},
		{"reason too long", id, id, id, domain.ValidationStatusRejected, strings.Repeat("a", 501), false},
		{"empty reason", id, id, id, domain.ValidationStatusRejected, "", false},
		{"bad event", "bad", id, id, domain.ValidationStatusValidated, "", false},
		{"bad resume", id, "", id, domain.ValidationStatusValidated, "", false},
		{"bad applicant", id, id, "bad", domain.ValidationStatusValidated, "", false},
		{"pending is not result", id, id, id, domain.ValidationStatusPending, "", false},
		{"unspecified", id, id, id, domain.ValidationStatusUnspecified, "", false},
		{"validated cannot carry reason", id, id, id, domain.ValidationStatusValidated, "bad", false},
		{"NUL cannot be stored", id, id, id, domain.ValidationStatusRejected, "a\x00b", false},
		{"invalid protobuf text", id, id, id, domain.ValidationStatusRejected, string([]byte{255}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NewValidationResult(tc.event, tc.resume, tc.applicant, tc.status, tc.reason)
			if !tc.valid {
				require.ErrorIs(t, err, domain.ErrInvalidValidationResult)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, strings.ToLower(id), got.EventID)
			require.Equal(t, strings.ToLower(id), got.ResumeID)
			require.Equal(t, strings.ToLower(id), got.ApplicantID)
			require.Equal(t, strings.TrimSpace(tc.reason), got.Reason)
		})
	}
}
