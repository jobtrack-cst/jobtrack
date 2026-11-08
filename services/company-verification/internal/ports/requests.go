package ports

//go:generate go tool mockgen -destination=mocks/requests_mock.go -package=mocks . RequestRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

// RequestRepository stores verification requests.
// This service calls no other services in Sprint 1, so there are no client
// ports. Publishing employer.verified and employer.verification_rejected
// uses EventPublisher in Sprint 2.
type RequestRepository interface {
	// Create stores a new PENDING request. It returns
	// domain.ErrPendingRequestExists if the employer already has a PENDING
	// request, including when two submits race.
	Create(ctx context.Context, r domain.VerificationRequest) (domain.VerificationRequest, error)
	// StatusesByEmployer returns the status of every request the employer
	// has made, for domain.CheckCanSubmit. Empty when there are none.
	StatusesByEmployer(ctx context.Context, employerID string) ([]domain.RequestStatus, error)
	// GetByID returns domain.ErrRequestNotFound for an unknown request.
	GetByID(ctx context.Context, id string) (domain.VerificationRequest, error)
	// ListByEmployer returns one page of the employer's requests, newest
	// first, and the total count.
	ListByEmployer(ctx context.Context, employerID string, page domain.Page) ([]domain.VerificationRequest, int64, error)
	// List returns one page of all requests, oldest first so admins work
	// the queue in order, and the total count. domain.StatusUnspecified
	// means all statuses.
	List(ctx context.Context, status domain.RequestStatus, page domain.Page) ([]domain.VerificationRequest, int64, error)
	// Decide writes status, rejection_reason, reviewed_by and updated_at only
	// while the stored row is still PENDING, so two admins cannot both
	// decide. It returns domain.ErrRequestNotPending if the row was already
	// decided, or domain.ErrRequestNotFound if it does not exist.
	Decide(ctx context.Context, r domain.VerificationRequest) (domain.VerificationRequest, error)
}

// Clock makes created_at and updated_at testable.
type Clock interface {
	Now() time.Time
}
