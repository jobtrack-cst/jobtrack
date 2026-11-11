package app

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports"
)

// Statuses holds the application status use cases. The gRPC adapter builds
// the Caller from pkg/authctx and translates domain errors to gRPC codes.
//
// Start and Transition have no RPC: in Sprint 2 the event consumers
// (application.submitted, review.completed, interview.scheduled, offer.*,
// application.withdrawn) call them, and Transition publishes
// application.status_changed where marked below.
type Statuses struct {
	repo ports.StatusRepository
}

func NewStatuses(repo ports.StatusRepository) *Statuses {
	return &Statuses{repo: repo}
}

// HistoryPage is one page of status history.
type HistoryPage struct {
	Entries       []domain.HistoryEntry
	TotalCount    int
	NextPageToken string
}

// Start records a new application as SUBMITTED, with its first history
// entry. It returns domain.ErrApplicationExists if it is already recorded.
func (s *Statuses) Start(ctx context.Context, applicationID, applicantID, employerID string, at time.Time) (domain.ApplicationStatus, error) {
	status, first, err := domain.NewSubmitted(applicationID, applicantID, employerID, at)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	if err := s.repo.Create(ctx, status, first); err != nil {
		return domain.ApplicationStatus{}, err
	}
	return status, nil
}

// Transition moves an application to the next status and stores the history
// entry. Any change AGENTS.md Section 6 does not allow returns
// domain.ErrInvalidTransition (FAILED_PRECONDITION) and stores nothing.
func (s *Statuses) Transition(ctx context.Context, rawApplicationID string, to domain.Status, at time.Time) (domain.ApplicationStatus, error) {
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	current, err := s.repo.Get(ctx, applicationID)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	updated, entry, err := current.Transition(to, at)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	// The repository only writes if the status is still current.Status, so
	// of two concurrent transitions only one is stored.
	if err := s.repo.Transition(ctx, updated, current.Status, entry); err != nil {
		return domain.ApplicationStatus{}, err
	}
	// Sprint 2: publish application.status_changed here, after it is stored.
	return updated, nil
}

// GetStatus returns the current status to the application's applicant or
// employer. The role is checked before the lookup, so admins learn nothing.
func (s *Statuses) GetStatus(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.ApplicationStatus, error) {
	return s.readable(ctx, caller, rawApplicationID)
}

// ListHistory returns the application's history, oldest first, to its
// applicant or employer.
func (s *Statuses) ListHistory(ctx context.Context, caller domain.Caller, rawApplicationID string, pageSize int32, pageToken string) (HistoryPage, error) {
	if err := caller.RequireParticipantRole(); err != nil {
		return HistoryPage{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return HistoryPage{}, err
	}
	status, err := s.readable(ctx, caller, rawApplicationID)
	if err != nil {
		return HistoryPage{}, err
	}
	entries, total, err := s.repo.ListHistory(ctx, status.ApplicationID, page)
	if err != nil {
		return HistoryPage{}, err
	}
	return HistoryPage{Entries: entries, TotalCount: total, NextPageToken: page.NextPageToken(total)}, nil
}

// readable checks the role, the ID, that the application exists and that the
// caller is its applicant or employer, in that order.
func (s *Statuses) readable(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.ApplicationStatus, error) {
	if err := caller.RequireParticipantRole(); err != nil {
		return domain.ApplicationStatus{}, err
	}
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	status, err := s.repo.Get(ctx, applicationID)
	if err != nil {
		return domain.ApplicationStatus{}, err
	}
	if err := status.CheckReader(caller); err != nil {
		return domain.ApplicationStatus{}, err
	}
	return status, nil
}
