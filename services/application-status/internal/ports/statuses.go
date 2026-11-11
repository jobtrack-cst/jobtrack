package ports

//go:generate go tool mockgen -destination=mocks/statuses_mock.go -package=mocks . StatusRepository

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

// StatusRepository stores each application's current status and its history.
// Every write that changes the status also appends its history entry in the
// same transaction, so the two can never disagree.
type StatusRepository interface {
	// Create stores a new SUBMITTED status with its first history entry.
	// It returns domain.ErrApplicationExists if the application is already recorded.
	Create(ctx context.Context, status domain.ApplicationStatus, first domain.HistoryEntry) error
	// Get returns the current status, or domain.ErrApplicationNotFound.
	Get(ctx context.Context, applicationID string) (domain.ApplicationStatus, error)
	// Transition writes updated only if the stored status still equals
	// expected, and appends entry. It returns domain.ErrApplicationNotFound,
	// or domain.ErrStatusChanged after a concurrent change.
	Transition(ctx context.Context, updated domain.ApplicationStatus, expected domain.Status, entry domain.HistoryEntry) error
	// ListHistory lists one application's entries oldest first (insertion
	// order breaks ties). It also returns the total count.
	ListHistory(ctx context.Context, applicationID string, page domain.Page) ([]domain.HistoryEntry, int, error)
}
