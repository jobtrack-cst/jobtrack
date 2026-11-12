package ports

//go:generate go tool mockgen -destination=mocks/verifications_mock.go -package=mocks . VerificationRepository

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

// VerificationRepository stores verification decisions from company-verification
// events (JT-56).
type VerificationRepository interface {
	// ApplyVerification claims the event_id and stores the decision in one
	// transaction. A duplicate event_id or a decision older than the stored
	// one changes nothing and is not an error. If the employer has a profile,
	// its status, reason and updated_at (set to now) are updated too; a
	// failed write marks nothing processed.
	ApplyVerification(ctx context.Context, d domain.VerificationDecision, now time.Time) (domain.VerificationOutcome, error)
}
