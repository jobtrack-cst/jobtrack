package domain

import "time"

// HiringKind is one hiring event that application-status consumes (JT-81).
type HiringKind uint8

const (
	HiringUnspecified HiringKind = iota
	HiringReviewShortlisted
	HiringReviewRejected
	HiringInterviewScheduled
	HiringOfferMade
	HiringOfferAccepted
	HiringOfferDeclined
)

// HiringEvent is validated input from review, interview or offer. ApplicantID
// is set only for the offer events, which carry it.
type HiringEvent struct {
	EventID       string
	ApplicationID string
	ApplicantID   string
	Kind          HiringKind
	OccurredAt    time.Time
}

var hiringTarget = map[HiringKind]Status{
	HiringReviewShortlisted:  StatusShortlisted,
	HiringReviewRejected:     StatusRejected,
	HiringInterviewScheduled: StatusInterview,
	HiringOfferMade:          StatusOffered,
	HiringOfferAccepted:      StatusAccepted,
	HiringOfferDeclined:      StatusDeclined,
}

// ApplyHiring returns the updated status and the history entries one hiring
// event causes, following the rules agreed in JT-81:
//   - review SHORTLISTED from SUBMITTED moves through UNDER_REVIEW, so it
//     stores two entries; every other event is one Section 6 transition;
//   - interview.scheduled when the status is already INTERVIEW (a further
//     interview) changes nothing and returns no entries;
//   - any other transition Section 6 does not allow is ErrInvalidTransition;
//   - an offer event whose applicant differs from the stored one is
//     ErrConflictingEvent.
func (a ApplicationStatus) ApplyHiring(e HiringEvent) (ApplicationStatus, []HistoryEntry, error) {
	target, ok := hiringTarget[e.Kind]
	if !ok {
		return ApplicationStatus{}, nil, ErrConflictingEvent
	}
	if e.ApplicantID != "" && e.ApplicantID != a.ApplicantID {
		return ApplicationStatus{}, nil, ErrConflictingEvent
	}
	if e.Kind == HiringInterviewScheduled && a.Status == StatusInterview {
		return a, nil, nil
	}
	steps := []Status{target}
	if e.Kind == HiringReviewShortlisted && a.Status == StatusSubmitted {
		steps = []Status{StatusUnderReview, StatusShortlisted}
	}
	var entries []HistoryEntry
	for _, to := range steps {
		next, entry, err := a.Transition(to, e.OccurredAt)
		if err != nil {
			return ApplicationStatus{}, nil, err
		}
		a = next
		entries = append(entries, entry)
	}
	return a, entries, nil
}
