package domain

import "time"

// Status is the application lifecycle from AGENTS.md Section 6:
//
//	SUBMITTED -> UNDER_REVIEW -> SHORTLISTED -> INTERVIEW -> OFFERED -> ACCEPTED
//	                                                                 \-> DECLINED
//	REJECTED: allowed from SUBMITTED, UNDER_REVIEW, SHORTLISTED, INTERVIEW
//	WITHDRAWN: allowed from any non-final state
//	Final states: ACCEPTED, DECLINED, REJECTED, WITHDRAWN
type Status uint8

const (
	StatusUnspecified Status = iota
	StatusSubmitted
	StatusUnderReview
	StatusShortlisted
	StatusInterview
	StatusOffered
	StatusAccepted
	StatusDeclined
	StatusRejected
	StatusWithdrawn
)

// AllStatuses lists every real status, in lifecycle order.
var AllStatuses = []Status{
	StatusSubmitted, StatusUnderReview, StatusShortlisted, StatusInterview, StatusOffered,
	StatusAccepted, StatusDeclined, StatusRejected, StatusWithdrawn,
}

// String returns the value stored in the database status columns.
func (s Status) String() string {
	switch s {
	case StatusSubmitted:
		return "submitted"
	case StatusUnderReview:
		return "under_review"
	case StatusShortlisted:
		return "shortlisted"
	case StatusInterview:
		return "interview"
	case StatusOffered:
		return "offered"
	case StatusAccepted:
		return "accepted"
	case StatusDeclined:
		return "declined"
	case StatusRejected:
		return "rejected"
	case StatusWithdrawn:
		return "withdrawn"
	default:
		return ""
	}
}

// ParseStatus reads a stored status value.
func ParseStatus(s string) (Status, bool) {
	for _, st := range AllStatuses {
		if st.String() == s {
			return st, true
		}
	}
	return StatusUnspecified, false
}

// IsFinal reports whether no further transition is possible.
func (s Status) IsFinal() bool {
	switch s {
	case StatusAccepted, StatusDeclined, StatusRejected, StatusWithdrawn:
		return true
	default:
		return false
	}
}

// forward is the main path; each status has at most these next steps
// besides REJECTED and WITHDRAWN.
var forward = map[Status][]Status{
	StatusSubmitted:   {StatusUnderReview},
	StatusUnderReview: {StatusShortlisted},
	StatusShortlisted: {StatusInterview},
	StatusInterview:   {StatusOffered},
	StatusOffered:     {StatusAccepted, StatusDeclined},
}

// CanTransition reports whether from -> to is allowed. Staying in the same
// status is not a transition, and nothing leaves a final state.
func CanTransition(from, to Status) bool {
	if from == StatusUnspecified || from.IsFinal() {
		return false
	}
	switch to {
	case StatusRejected:
		return from != StatusOffered
	case StatusWithdrawn:
		return true
	}
	for _, next := range forward[from] {
		if next == to {
			return true
		}
	}
	return false
}

// ApplicationStatus is the current status of one application, with the
// participants who may read it.
type ApplicationStatus struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
	Status        Status
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// HistoryEntry records one change. The first entry of every application has
// PreviousStatus StatusUnspecified and NewStatus StatusSubmitted.
type HistoryEntry struct {
	ApplicationID  string
	PreviousStatus Status
	NewStatus      Status
	OccurredAt     time.Time
}

// NewSubmitted starts an application's lifecycle at SUBMITTED and returns the
// first history entry. at is when the application was submitted.
func NewSubmitted(applicationID, applicantID, employerID string, at time.Time) (ApplicationStatus, HistoryEntry, error) {
	appID, err := NormalizeApplicationID(applicationID)
	if err != nil {
		return ApplicationStatus{}, HistoryEntry{}, err
	}
	applicant, ok := normalizeUUID(applicantID)
	if !ok {
		return ApplicationStatus{}, HistoryEntry{}, ErrInvalidParticipantID
	}
	employer, ok := normalizeUUID(employerID)
	if !ok {
		return ApplicationStatus{}, HistoryEntry{}, ErrInvalidParticipantID
	}
	if at.IsZero() {
		return ApplicationStatus{}, HistoryEntry{}, ErrInvalidTime
	}
	at = at.UTC()
	return ApplicationStatus{
			ApplicationID: appID, ApplicantID: applicant, EmployerID: employer,
			Status: StatusSubmitted, CreatedAt: at, UpdatedAt: at,
		},
		HistoryEntry{ApplicationID: appID, PreviousStatus: StatusUnspecified, NewStatus: StatusSubmitted, OccurredAt: at},
		nil
}

// Transition moves the application to the next status. It returns the
// updated status and the history entry to store with it, or
// ErrInvalidTransition for any change Section 6 does not allow.
func (a ApplicationStatus) Transition(to Status, at time.Time) (ApplicationStatus, HistoryEntry, error) {
	if !CanTransition(a.Status, to) {
		return ApplicationStatus{}, HistoryEntry{}, ErrInvalidTransition
	}
	if at.IsZero() {
		return ApplicationStatus{}, HistoryEntry{}, ErrInvalidTime
	}
	at = at.UTC()
	entry := HistoryEntry{ApplicationID: a.ApplicationID, PreviousStatus: a.Status, NewStatus: to, OccurredAt: at}
	a.Status, a.UpdatedAt = to, at
	return a, entry, nil
}

// CheckReader allows only the application's applicant (as an applicant) or
// its employer (as an employer).
func (a ApplicationStatus) CheckReader(c Caller) error {
	switch {
	case c.Role == RoleApplicant && c.UserID == a.ApplicantID:
		return nil
	case c.Role == RoleEmployer && c.UserID == a.EmployerID:
		return nil
	default:
		return ErrNotParticipant
	}
}
