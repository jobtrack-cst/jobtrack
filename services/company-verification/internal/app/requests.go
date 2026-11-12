package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
)

// Requests holds the verification request use cases. The gRPC adapter builds
// the Caller from pkg/authctx and translates domain errors to gRPC codes.
// Every use case checks the role first, then the input, then stored state,
// so a caller with the wrong role learns nothing else.
type Requests struct {
	repo       ports.RequestRepository
	clock      ports.Clock
	newEventID func() string
}

func NewRequests(repo ports.RequestRepository, clock ports.Clock) *Requests {
	return &Requests{repo: repo, clock: clock, newEventID: uuid.NewString}
}

// ListResult is one page of requests with the values the proto pagination needs.
type ListResult struct {
	Requests      []domain.VerificationRequest
	TotalCount    int64
	NextPageToken string
}

// Submit creates a PENDING request for the calling employer. The employer ID
// always comes from the caller, never from the input.
func (s *Requests) Submit(ctx context.Context, caller domain.Caller, registrationNumber, notes string) (domain.VerificationRequest, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.VerificationRequest{}, err
	}
	sub, err := domain.NewSubmission(registrationNumber, notes)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	statuses, err := s.repo.StatusesByEmployer(ctx, caller.UserID)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	if err := domain.CheckCanSubmit(statuses); err != nil {
		return domain.VerificationRequest{}, err
	}
	r := domain.NewVerificationRequest(uuid.NewString(), caller.UserID, sub, s.clock.Now())
	return s.repo.Create(ctx, r)
}

// ListMine returns the calling employer's own requests, newest first.
func (s *Requests) ListMine(ctx context.Context, caller domain.Caller, pageSize int32, pageToken string) (ListResult, error) {
	if err := caller.RequireEmployer(); err != nil {
		return ListResult{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return ListResult{}, err
	}
	items, total, err := s.repo.ListByEmployer(ctx, caller.UserID, page)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Requests: items, TotalCount: total, NextPageToken: page.NextToken(len(items), total)}, nil
}

// List returns all requests for the admin, optionally filtered by status.
func (s *Requests) List(ctx context.Context, caller domain.Caller, status domain.RequestStatus, pageSize int32, pageToken string) (ListResult, error) {
	if err := caller.RequireAdmin(); err != nil {
		return ListResult{}, err
	}
	if !status.ValidFilter() {
		return ListResult{}, domain.ErrInvalidStatusFilter
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return ListResult{}, err
	}
	items, total, err := s.repo.List(ctx, status, page)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Requests: items, TotalCount: total, NextPageToken: page.NextToken(len(items), total)}, nil
}

// Approve decides a PENDING request and stores employer.verified with it.
func (s *Requests) Approve(ctx context.Context, caller domain.Caller, requestID string) (domain.VerificationRequest, error) {
	if err := caller.RequireAdmin(); err != nil {
		return domain.VerificationRequest{}, err
	}
	id, err := domain.NormalizeID(requestID, domain.ErrInvalidRequestID)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	return s.decide(ctx, id, func(r *domain.VerificationRequest) error {
		return r.Approve(caller.UserID, s.clock.Now())
	})
}

// Reject decides a PENDING request with a reason and stores
// employer.verification_rejected with it.
func (s *Requests) Reject(ctx context.Context, caller domain.Caller, requestID, reason string) (domain.VerificationRequest, error) {
	if err := caller.RequireAdmin(); err != nil {
		return domain.VerificationRequest{}, err
	}
	id, err := domain.NormalizeID(requestID, domain.ErrInvalidRequestID)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	why, err := domain.NewRejectionReason(reason)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	return s.decide(ctx, id, func(r *domain.VerificationRequest) error {
		return r.Reject(caller.UserID, why, s.clock.Now())
	})
}

// decide loads the request so the domain can refuse a decided one early, and
// the repository repeats the PENDING check in the UPDATE for concurrent admins.
// The event is stored in the same transaction as the decision; the outbox
// relay publishes it after the commit.
func (s *Requests) decide(ctx context.Context, id string, apply func(*domain.VerificationRequest) error) (domain.VerificationRequest, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	if err := apply(&r); err != nil {
		return domain.VerificationRequest{}, err
	}
	event, err := DecisionEvent(r, s.newEventID())
	if err != nil {
		return domain.VerificationRequest{}, err
	}
	return s.repo.Decide(ctx, r, event)
}
