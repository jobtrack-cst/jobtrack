package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
)

const (
	validUUID1 = "11111111-1111-4111-8111-111111111111"
	validUUID2 = "22222222-2222-4222-8222-222222222222"
)

func TestNewNotification(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		recipientID   string
		nType         string
		title         string
		body          string
		sourceEventID string
		wantErr       error
	}{
		{"valid", validUUID1, "offer.made", "New Offer", "You have an offer", validUUID2, nil},
		{"invalid recipient", "not-a-uuid", "offer.made", "Title", "Body", validUUID2, domain.ErrInvalidRecipientID},
		{"invalid event", validUUID1, "offer.made", "Title", "Body", "nope", domain.ErrInvalidSourceEventID},
		{"empty type", validUUID1, "  ", "Title", "Body", validUUID2, domain.ErrInvalidType},
		{"empty title", validUUID1, "offer.made", "", "Body", validUUID2, domain.ErrInvalidTitle},
		{"long title", validUUID1, "offer.made", strings.Repeat("a", 151), "Body", validUUID2, domain.ErrInvalidTitle},
		{"long body", validUUID1, "offer.made", "Title", strings.Repeat("a", 1001), validUUID2, domain.ErrInvalidBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewNotification(tt.recipientID, tt.nType, tt.title, tt.body, "", tt.sourceEventID, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewNotification() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
