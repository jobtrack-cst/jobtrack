package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports"
)

// ApplicationParticipants lets the owning employer of an application read the
// applicant's profile and photo (JT-114 Part C). Access is decided only by the
// local application.submitted projection, never by request input.
type ApplicationParticipants struct {
	participants ports.ApplicationParticipantRepository
	profiles     ports.ProfileRepository
}

func NewApplicationParticipants(participants ports.ApplicationParticipantRepository, profiles ports.ProfileRepository) *ApplicationParticipants {
	return &ApplicationParticipants{participants: participants, profiles: profiles}
}

// Profile returns the applicant's current profile. Like every profile read,
// it never loads the photo bytes (JT-103); HasPhoto tells whether one exists.
func (a *ApplicationParticipants) Profile(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.Profile, error) {
	row, err := a.authorize(ctx, caller, rawApplicationID)
	if err != nil {
		return domain.Profile{}, err
	}
	return a.profiles.Get(ctx, row.ApplicantID)
}

// Photo returns the applicant's current photo under the same access rule.
func (a *ApplicationParticipants) Photo(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.Photo, error) {
	row, err := a.authorize(ctx, caller, rawApplicationID)
	if err != nil {
		return domain.Photo{}, err
	}
	return a.profiles.GetPhoto(ctx, row.ApplicantID)
}

// authorize applies the JT-114 rules in order: employer role (before input),
// UUID input, known application, owning employer. Access stays after a
// withdrawal, so the employer keeps the record of who applied.
func (a *ApplicationParticipants) authorize(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.ApplicationParticipant, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.ApplicationParticipant{}, err
	}
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return domain.ApplicationParticipant{}, err
	}
	row, err := a.participants.GetByApplicationID(ctx, applicationID)
	if err != nil {
		return domain.ApplicationParticipant{}, err
	}
	if err := row.CheckEmployer(caller); err != nil {
		return domain.ApplicationParticipant{}, err
	}
	return row, nil
}
