package domain

import "errors"

// JT-114 Part B errors. Each maps to one gRPC code in the gRPC adapter.
var (
	// PERMISSION_DENIED
	ErrEmployerRoleRequired   = errors.New("only employers can download application resumes")
	ErrNotApplicationEmployer = errors.New("only the employer of this application can download its resume")

	// INVALID_ARGUMENT
	ErrInvalidApplicationID = errors.New("application_id must be a UUID")

	// NOT_FOUND: unknown, or application.submitted not processed yet.
	ErrApplicationNotFound = errors.New("application not found")

	// Consumer only: an application.submitted event that can never be stored.
	ErrInvalidApplicationEvent = errors.New("invalid application submitted event")
)

// ApplicationResume is the trusted local projection of application.submitted:
// who applied, to which employer, with which resume. All IDs are lowercase.
type ApplicationResume struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
	ResumeID      string
}

// NormalizeApplicationID validates an application_id from a request or event.
func NormalizeApplicationID(raw string) (string, error) {
	id, ok := normalizeUUID(raw)
	if !ok {
		return "", ErrInvalidApplicationID
	}
	return id, nil
}

// NormalizeEventID validates and lowercases an event_id before it is claimed.
func NormalizeEventID(raw string) (string, error) {
	id, ok := normalizeUUID(raw)
	if !ok {
		return "", ErrInvalidApplicationEvent
	}
	return id, nil
}

// NewApplicationResume validates and lowercases every ID from an event.
func NewApplicationResume(applicationID, applicantID, employerID, resumeID string) (ApplicationResume, error) {
	ids := []string{applicationID, applicantID, employerID, resumeID}
	for i, raw := range ids {
		id, ok := normalizeUUID(raw)
		if !ok {
			return ApplicationResume{}, ErrInvalidApplicationEvent
		}
		ids[i] = id
	}
	return ApplicationResume{ApplicationID: ids[0], ApplicantID: ids[1], EmployerID: ids[2], ResumeID: ids[3]}, nil
}

// RequireEmployer guards DownloadApplicationResume before any input is read.
func (c Caller) RequireEmployer() error {
	if c.Role != RoleEmployer {
		return ErrEmployerRoleRequired
	}
	return nil
}

// CheckEmployer allows access only to the application's owning employer.
func (a ApplicationResume) CheckEmployer(c Caller) error {
	if a.EmployerID != c.UserID {
		return ErrNotApplicationEmployer
	}
	return nil
}
