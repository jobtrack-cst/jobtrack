package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
)

// Verifications applies employer.verified and employer.verification_rejected
// events (JT-56). No user is involved, so there is no caller check.
type Verifications struct {
	repo  ports.VerificationRepository
	clock ports.Clock
}

func NewVerifications(repo ports.VerificationRepository, clock ports.Clock) *Verifications {
	return &Verifications{repo: repo, clock: clock}
}

// Apply validates the decision again, so a caller that skipped
// domain.NewVerificationDecision cannot store an invalid one, then stores it.
func (v *Verifications) Apply(ctx context.Context, d domain.VerificationDecision) (domain.VerificationOutcome, error) {
	valid, err := domain.NewVerificationDecision(d.EventID, d.EmployerID, d.Status, d.RejectionReason, d.DecidedAt)
	if err != nil {
		return 0, err
	}
	return v.repo.ApplyVerification(ctx, valid, v.clock.Now())
}
