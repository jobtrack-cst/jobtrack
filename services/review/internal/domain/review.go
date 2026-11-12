package domain

import "time"

// Score limits from the review.proto contract.
const (
	MinScore = 1
	MaxScore = 5
)

// Decision is the employer's shortlist decision. Scores do not decide it.
type Decision uint8

const (
	DecisionUnspecified Decision = iota
	DecisionShortlisted
	DecisionRejected
)

// String returns the value stored in the database and, in Sprint 2, the
// decision string in review.completed.
func (d Decision) String() string {
	switch d {
	case DecisionShortlisted:
		return "SHORTLISTED"
	case DecisionRejected:
		return "REJECTED"
	default:
		return ""
	}
}

// ParseDecision reads a stored decision.
func ParseDecision(s string) (Decision, bool) {
	for _, d := range []Decision{DecisionShortlisted, DecisionRejected} {
		if d.String() == s {
			return d, true
		}
	}
	return DecisionUnspecified, false
}

// ApplicationParticipants is the trusted local projection of
// application.submitted (filled by the Sprint 2 consumer), the same design as
// Offer and Messaging. EmployerID is the employer's user ID.
type ApplicationParticipants struct {
	ApplicationID string
	ApplicantID   string
	EmployerID    string
}

// CheckOwningEmployer allows only the employer stored for the application.
func (p ApplicationParticipants) CheckOwningEmployer(c Caller) error {
	if c.Role != RoleEmployer || c.UserID != p.EmployerID {
		return ErrNotOwningEmployer
	}
	return nil
}

// Review is one completed review. There is at most one per application.
type Review struct {
	ReviewID      string
	ApplicationID string
	EmployerID    string
	Score         int32
	Decision      Decision
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ReviewInput is the raw content of a CompleteReview request.
type ReviewInput struct {
	ApplicationID string
	Score         int32
	Decision      Decision
}

// ValidReviewInput is a checked request with a normalized application ID.
type ValidReviewInput struct {
	ApplicationID string
	Score         int32
	Decision      Decision
}

// NewReviewInput applies the contract rules: a UUID application_id, a score
// from 1 to 5 and a decision other than UNSPECIFIED.
func NewReviewInput(in ReviewInput) (ValidReviewInput, error) {
	applicationID, err := NormalizeApplicationID(in.ApplicationID)
	if err != nil {
		return ValidReviewInput{}, err
	}
	if in.Score < MinScore || in.Score > MaxScore {
		return ValidReviewInput{}, ErrInvalidScore
	}
	if in.Decision != DecisionShortlisted && in.Decision != DecisionRejected {
		return ValidReviewInput{}, ErrInvalidDecision
	}
	return ValidReviewInput{ApplicationID: applicationID, Score: in.Score, Decision: in.Decision}, nil
}
