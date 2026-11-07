package domain

import (
	"strings"
	"testing"
	"time"
)

func TestApplication_ValidID(t *testing.T) {
	for _, tt := range []struct {
		id   string
		want bool
	}{
		{"a1b2c3d4-1111-4111-8111-111111111111", true}, {"", false}, {"invalid", false}, {"00000000-0000-0000-0000-000000000000", false}, {"A1B2C3D4-1111-4111-8111-111111111111", false}, {strings.Repeat("x", 10000), false},
	} {
		t.Run(tt.id[:min(len(tt.id), 40)], func(t *testing.T) {
			if ValidID(tt.id) != tt.want {
				t.Fatal("unexpected UUID validity")
			}
		})
	}
}
func TestJob_Eligible_DeadlineBoundary(t *testing.T) {
	now := time.Now()
	for _, tt := range []struct {
		name      string
		published bool
		deadline  time.Time
		want      bool
	}{
		{"open", true, now.Add(time.Nanosecond), true}, {"at deadline", true, now, false}, {"expired", true, now.Add(-time.Nanosecond), false}, {"closed", false, now.Add(time.Hour), false}, {"missing deadline", true, time.Time{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if (Job{Published: tt.published, Deadline: tt.deadline}).Eligible(now) != tt.want {
				t.Fatal("unexpected eligibility")
			}
		})
	}
}
