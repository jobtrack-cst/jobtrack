package ports

//go:generate go tool mockgen -destination=mocks/interviews_mock.go -package=mocks . ParticipantRepository,InterviewRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
)

// ParticipantRepository reads the local trusted projection of
// application.submitted. The Sprint 2 consumer adds the method that fills it.
type ParticipantRepository interface {
	// GetByApplicationID returns domain.ErrApplicationNotFound for an unknown
	// or not yet processed application.
	GetByApplicationID(ctx context.Context, applicationID string) (domain.ApplicationParticipants, error)
}

// InterviewRepository stores interviews, several per application.
type InterviewRepository interface {
	// Create stores the interview, or returns domain.ErrInterviewExists if
	// the application already has one at the same scheduled_at.
	Create(ctx context.Context, interview domain.Interview) error
	// Get returns the interview, or domain.ErrInterviewNotFound.
	Get(ctx context.Context, interviewID string) (domain.Interview, error)
	// ListByApplication lists one application's interviews soonest first
	// (interview_id breaks ties). It also returns the total count.
	ListByApplication(ctx context.Context, applicationID string, page domain.Page) ([]domain.Interview, int, error)
}

// Clock supplies "now" for the future-time rule and the timestamps.
type Clock interface {
	Now() time.Time
}
