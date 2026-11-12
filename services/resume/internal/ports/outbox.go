package ports

//go:generate go tool mockgen -destination=mocks/outbox_mock.go -package=mocks . OutboxRepository

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

// OutboxRepository persists claims only; it performs no network publication.
type OutboxRepository interface {
	// Claim commits ownership of one eligible pending event before returning.
	// Candidates are ordered by created_at, event_id; locked rows are skipped.
	// found=false means no eligible unlocked event. Expired leases may be reclaimed.
	Claim(ctx context.Context, lease time.Duration) (event domain.OutgoingEvent, found bool, err error)
	// MarkPublished must only be called after confirmed publication. It clears the
	// claim and retry state. A stale token or published event returns false, nil.
	// Expiry permits reclamation, but does not invalidate an unreplaced token.
	MarkPublished(ctx context.Context, eventID, token string) (bool, error)
	// Retry clears the matching claim and schedules another attempt relative to
	// database time; zero delay releases it immediately. Stale tokens do nothing.
	Retry(ctx context.Context, eventID, token string, delay time.Duration) (bool, error)
}
