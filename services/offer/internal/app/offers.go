package app

import (
	"context"
	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports"
)

type Offers struct {
	offers       ports.OfferRepository
	participants ports.ParticipantRepository
	clock        ports.Clock
}

func NewOffers(offers ports.OfferRepository, participants ports.ParticipantRepository, clock ports.Clock) *Offers {
	return &Offers{offers: offers, participants: participants, clock: clock}
}

func (s *Offers) MakeOffer(ctx context.Context, caller domain.Caller, applicationID, rawTerms string) (domain.Offer, error) {
	if err := caller.RequireRole("employer"); err != nil {
		return domain.Offer{}, err
	}
	id, err := domain.NormalizeID(applicationID, domain.ErrInvalidApplicationID)
	if err != nil {
		return domain.Offer{}, err
	}
	terms, err := domain.ValidateTerms(rawTerms)
	if err != nil {
		return domain.Offer{}, err
	}
	p, err := s.participants.GetByApplicationID(ctx, id)
	if err != nil {
		return domain.Offer{}, err
	}
	owner, _ := domain.NormalizeID(caller.UserID, domain.ErrUnauthenticated)
	if p.EmployerID != owner {
		return domain.Offer{}, domain.ErrPermissionDenied
	}
	now := s.clock.Now().UTC()
	return s.offers.Create(ctx, domain.Offer{ID: uuid.NewString(), ApplicationID: p.ApplicationID, ApplicantID: p.ApplicantID, EmployerID: p.EmployerID, Terms: terms, Status: domain.OfferStatusPending, CreatedAt: now, UpdatedAt: now})
}

func (s *Offers) GetOffer(ctx context.Context, caller domain.Caller, offerID string) (domain.Offer, error) {
	if _, err := domain.NewCaller(caller.UserID, caller.Role); err != nil {
		return domain.Offer{}, err
	}
	id, err := domain.NormalizeID(offerID, domain.ErrInvalidOfferID)
	if err != nil {
		return domain.Offer{}, err
	}
	o, err := s.offers.GetByID(ctx, id)
	if err != nil {
		return domain.Offer{}, err
	}
	if err := o.CanView(caller); err != nil {
		return domain.Offer{}, err
	}
	return o, nil
}

func (s *Offers) AcceptOffer(ctx context.Context, caller domain.Caller, id string) (domain.Offer, error) {
	return s.decide(ctx, caller, id, true)
}
func (s *Offers) DeclineOffer(ctx context.Context, caller domain.Caller, id string) (domain.Offer, error) {
	return s.decide(ctx, caller, id, false)
}
func (s *Offers) decide(ctx context.Context, caller domain.Caller, offerID string, accept bool) (domain.Offer, error) {
	if err := caller.RequireRole("applicant"); err != nil {
		return domain.Offer{}, err
	}
	o, err := s.GetOffer(ctx, caller, offerID)
	if err != nil {
		return domain.Offer{}, err
	}
	if accept {
		err = o.Accept()
	} else {
		err = o.Decline()
	}
	if err != nil {
		return domain.Offer{}, err
	}
	return s.offers.Decide(ctx, o.ID, o.ApplicantID, o.Status, s.clock.Now().UTC())
}
