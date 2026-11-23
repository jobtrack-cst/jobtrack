package domain

import "errors"

// JT-114 Part C errors. Each maps to one gRPC code in the gRPC adapter.
var (
	// PERMISSION_DENIED
	ErrEmployerRoleRequired   = errors.New("only employers can read an application's applicant profile")
	ErrNotApplicationEmployer = errors.New("only the employer of this application can read its applicant profile")

	// INVALID_ARGUMENT
	ErrInvalidApplicationID = errors.New("application_id must be a UUID")

	// NOT_FOUND: unknown, or application.submitted not processed yet.
	ErrApplicationNotFound = errors.New("application not found")

	// Consumer only: an application.submitted event that can never be stored.
	ErrInvalidApplicationEvent = errors.New("invalid application submitted event")
)

// ApplicationParticipant is the trusted local projection of
// application.submitted: who applied to which employer. All IDs are lowercase.
type ApplicationParticipant struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
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

// IsUUID reports whether raw is a UUID. The consumer checks resume_id with it:
// the ID is not stored here, but the event must be valid as a whole.
func IsUUID(raw string) bool {
	_, ok := normalizeUUID(raw)
	return ok
}

// NewApplicationParticipant validates and lowercases every ID from an event.
func NewApplicationParticipant(applicationID, applicantID, employerID string) (ApplicationParticipant, error) {
	ids := []string{applicationID, applicantID, employerID}
	for i, raw := range ids {
		id, ok := normalizeUUID(raw)
		if !ok {
			return ApplicationParticipant{}, ErrInvalidApplicationEvent
		}
		ids[i] = id
	}
	return ApplicationParticipant{ApplicationID: ids[0], ApplicantID: ids[1], EmployerID: ids[2]}, nil
}

// RequireEmployer guards the two JT-114 RPCs before any input is read.
func (c Caller) RequireEmployer() error {
	if c.Role != RoleEmployer {
		return ErrEmployerRoleRequired
	}
	return nil
}

// CheckEmployer allows access only to the application's owning employer.
func (a ApplicationParticipant) CheckEmployer(c Caller) error {
	if a.EmployerID != c.UserID {
		return ErrNotApplicationEmployer
	}
	return nil
}
