package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/ports"
)

// Reviews holds the review use cases. The gRPC adapter builds the Caller from
// pkg/authctx and translates domain errors to gRPC codes. Ownership comes
// from the trusted application_participants projection (JT-51, the Offer
// design), never from the request.
type Reviews struct {
	participants ports.ParticipantRepository
	reviews      ports.ReviewRepository
	clock        ports.Clock
}

func NewReviews(participants ports.ParticipantRepository, reviews ports.ReviewRepository, clock ports.Clock) *Reviews {
	return &Reviews{participants: participants, reviews: reviews, clock: clock}
}

// Complete records the owning employer's review: role, then input, then that
// the application is known, then ownership, then one review per application.
func (r *Reviews) Complete(ctx context.Context, caller domain.Caller, in domain.ReviewInput) (domain.Review, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Review{}, err
	}
	valid, err := domain.NewReviewInput(in)
	if err != nil {
		return domain.Review{}, err
	}
	if err := r.checkOwner(ctx, caller, valid.ApplicationID); err != nil {
		return domain.Review{}, err
	}
	now := r.clock.Now().UTC()
	review := domain.Review{
		ReviewID: uuid.NewString(), ApplicationID: valid.ApplicationID, EmployerID: caller.UserID,
		Score: valid.Score, Decision: valid.Decision, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.reviews.Create(ctx, review); err != nil {
		return domain.Review{}, err
	}
	// Sprint 2: publish review.completed here, after the review is stored.
	return review, nil
}

// Get returns the review to the owning employer only (JT-51).
func (r *Reviews) Get(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.Review, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Review{}, err
	}
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return domain.Review{}, err
	}
	if err := r.checkOwner(ctx, caller, applicationID); err != nil {
		return domain.Review{}, err
	}
	return r.reviews.GetByApplicationID(ctx, applicationID)
}

func (r *Reviews) checkOwner(ctx context.Context, caller domain.Caller, applicationID string) error {
	participants, err := r.participants.GetByApplicationID(ctx, applicationID)
	if err != nil {
		return err
	}
	return participants.CheckOwningEmployer(caller)
}
