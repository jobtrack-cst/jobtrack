package ports

//go:generate go tool mockgen -destination=mocks/offers_mock.go -package=mocks . OfferRepository,Clock

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"time"
)

type OfferRepository interface {
	// Create stores offer.made in the same transaction and returns ErrOfferExists when application_id already has an offer.
	Create(context.Context, domain.Offer) (domain.Offer, error)
	// GetByID returns ErrOfferNotFound for an unknown offer.
	GetByID(context.Context, string) (domain.Offer, error)
	// Decide stores its decision event in the same transaction and atomically changes PENDING to ACCEPTED or DECLINED. The applicant
	// must match the stored owner. Later decisions return ErrOfferNotPending.
	Decide(context.Context, string, string, domain.OfferStatus, time.Time) (domain.Offer, error)
}

type Clock interface{ Now() time.Time }
