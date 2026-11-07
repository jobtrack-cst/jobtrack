package domain

import (
	"errors"
	"testing"
)

func TestOffer_DecisionTransitions(t *testing.T) {
	tests := []struct {
		name       string
		initial    OfferStatus
		action     string
		wantStatus OfferStatus
		wantErr    error
	}{
		{
			name:       "accept pending offer",
			initial:    OfferStatusPending,
			action:     "accept",
			wantStatus: OfferStatusAccepted,
		},
		{
			name:       "decline pending offer",
			initial:    OfferStatusPending,
			action:     "decline",
			wantStatus: OfferStatusDeclined,
		},
		{
			name:       "cannot accept twice",
			initial:    OfferStatusAccepted,
			action:     "accept",
			wantStatus: OfferStatusAccepted,
			wantErr:    ErrOfferNotPending,
		},
		{
			name:       "cannot decline twice",
			initial:    OfferStatusDeclined,
			action:     "decline",
			wantStatus: OfferStatusDeclined,
			wantErr:    ErrOfferNotPending,
		},
		{
			name:       "cannot decline accepted offer",
			initial:    OfferStatusAccepted,
			action:     "decline",
			wantStatus: OfferStatusAccepted,
			wantErr:    ErrOfferNotPending,
		},
		{
			name:       "cannot accept declined offer",
			initial:    OfferStatusDeclined,
			action:     "accept",
			wantStatus: OfferStatusDeclined,
			wantErr:    ErrOfferNotPending,
		},
		{
			name:       "cannot accept unspecified offer",
			initial:    OfferStatusUnspecified,
			action:     "accept",
			wantStatus: OfferStatusUnspecified,
			wantErr:    ErrOfferNotPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offer := &Offer{Status: tt.initial}

			var err error
			switch tt.action {
			case "accept":
				err = offer.Accept()
			case "decline":
				err = offer.Decline()
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}

			if offer.Status != tt.wantStatus {
				t.Errorf(
					"got status %v, want %v",
					offer.Status,
					tt.wantStatus,
				)
			}
		})
	}
}
