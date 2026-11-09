package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestOffer_ValidateTerms(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
		invalid         bool
	}{
		{"empty", "", "", true}, {"whitespace", " \n\t ", "", true},
		{"one character", " x ", "x", false},
		{"unicode boundary", strings.Repeat("ཀ", 2000), strings.Repeat("ཀ", 2000), false},
		{"over boundary", strings.Repeat("ཀ", 2001), "", true},
		{"trim unicode whitespace", "\u2003terms\u2003", "terms", false},
		{"embedded NUL", "terms\x00text", "", true},
		{"trailing NUL", "terms\x00", "", true},
		{"NUL only", "\x00", "", true},
		{"invalid UTF8", string([]byte{0xff}), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateTerms(tc.raw)
			if got != tc.want || (err != nil) != tc.invalid || (tc.invalid && !errors.Is(err, ErrInvalidTerms)) {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
func TestOffer_CallerValidation(t *testing.T) {
	for _, tc := range []struct {
		id, role string
		invalid  bool
	}{
		{"aaaaaaaa-1111-4111-8111-111111111111", "employer", false},
		{"AAAAAAAA-1111-4111-8111-111111111111", "applicant", false},
		{"aaaaaaaa-1111-4111-8111-111111111111", "admin", false},
		{"bad", "applicant", true}, {"", "employer", true},
		{"aaaaaaaa-1111-4111-8111-111111111111", "unknown", true},
	} {
		t.Run(tc.id+tc.role, func(t *testing.T) {
			got, err := NewCaller(tc.id, tc.role)
			if (err != nil) != tc.invalid {
				t.Fatal(got, err)
			}
			if !tc.invalid && got.UserID != strings.ToLower(tc.id) {
				t.Fatal(got)
			}
		})
	}
}
