package postgres

import (
	"context"
	"errors"
	"testing"
)

// Internal test: an empty status is stored as NULL, any other value as is.
func TestNullableStatus(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want any
	}{
		{"empty is NULL", "", nil},
		{"status is kept", "SHORTLISTED", "SHORTLISTED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := nullableStatus(tt.in); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafe_HidesDetailsButKeepsCancellation(t *testing.T) {
	err := safe(context.Background(), "count outcome events failed")
	if err == nil || err.Error() != "count outcome events failed" {
		t.Fatalf("err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := safe(ctx, "count outcome events failed"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
