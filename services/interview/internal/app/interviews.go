package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/ports"
)

// Interviews holds the interview use cases. The gRPC adapter builds the
// Caller from pkg/authctx and translates domain errors to gRPC codes.
// Ownership comes from the trusted application_participants projection
// (JT-52, the Review and Offer design), never from the request.
type Interviews struct {
	participants ports.ParticipantRepository
	interviews   ports.InterviewRepository
	clock        ports.Clock
}

func NewInterviews(participants ports.ParticipantRepository, interviews ports.InterviewRepository, clock ports.Clock) *Interviews {
	return &Interviews{participants: participants, interviews: interviews, clock: clock}
}

// InterviewPage is one page of a list result.
type InterviewPage struct {
	Interviews    []domain.Interview
	TotalCount    int
	NextPageToken string
}

// Schedule books an interview for the owning employer: role, then input
// (including the future-time rule), then that the application is known,
// then ownership. Two interviews at the same time are ALREADY_EXISTS.
func (s *Interviews) Schedule(ctx context.Context, caller domain.Caller, in domain.ScheduleInput) (domain.Interview, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Interview{}, err
	}
	now := s.clock.Now().UTC()
	valid, err := domain.NewSchedule(in, now)
	if err != nil {
		return domain.Interview{}, err
	}
	participants, err := s.participants.GetByApplicationID(ctx, valid.ApplicationID)
	if err != nil {
		return domain.Interview{}, err
	}
	if err := participants.CheckOwningEmployer(caller); err != nil {
		return domain.Interview{}, err
	}
	interview := domain.Interview{
		InterviewID: uuid.NewString(), ApplicationID: valid.ApplicationID,
		EmployerID: caller.UserID, ApplicantID: participants.ApplicantID,
		ScheduledAt: valid.ScheduledAt, Details: valid.Details, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.interviews.Create(ctx, interview); err != nil {
		return domain.Interview{}, err
	}
	// Sprint 2: publish interview.scheduled here, after the interview is stored.
	return interview, nil
}

// Get returns one interview to its applicant or owning employer. The
// interview stores both participants, so no second lookup is needed.
func (s *Interviews) Get(ctx context.Context, caller domain.Caller, rawInterviewID string) (domain.Interview, error) {
	if err := caller.RequireParticipantRole(); err != nil {
		return domain.Interview{}, err
	}
	interviewID, err := domain.NormalizeInterviewID(rawInterviewID)
	if err != nil {
		return domain.Interview{}, err
	}
	interview, err := s.interviews.Get(ctx, interviewID)
	if err != nil {
		return domain.Interview{}, err
	}
	readers := domain.ApplicationParticipants{
		ApplicationID: interview.ApplicationID, ApplicantID: interview.ApplicantID, EmployerID: interview.EmployerID,
	}
	if err := readers.CheckReader(caller); err != nil {
		return domain.Interview{}, err
	}
	return interview, nil
}

// List returns one application's interviews, soonest first, to its applicant
// or owning employer.
func (s *Interviews) List(ctx context.Context, caller domain.Caller, rawApplicationID string, pageSize int32, pageToken string) (InterviewPage, error) {
	if err := caller.RequireParticipantRole(); err != nil {
		return InterviewPage{}, err
	}
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return InterviewPage{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return InterviewPage{}, err
	}
	participants, err := s.participants.GetByApplicationID(ctx, applicationID)
	if err != nil {
		return InterviewPage{}, err
	}
	if err := participants.CheckReader(caller); err != nil {
		return InterviewPage{}, err
	}
	interviews, total, err := s.interviews.ListByApplication(ctx, applicationID, page)
	if err != nil {
		return InterviewPage{}, err
	}
	return InterviewPage{Interviews: interviews, TotalCount: total, NextPageToken: page.NextPageToken(total)}, nil
}
