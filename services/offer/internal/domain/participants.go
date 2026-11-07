package domain

import "errors"

var (
	ErrParticipantsNotFound = errors.New("application participants not found")
	ErrInvalidApplicationID = errors.New("application ID must be a UUID")
)

// ApplicationParticipants is the trusted local projection of application.submitted.
type ApplicationParticipants struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
}
