package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

const (
	hiringAppID     = "11111111-1111-4111-8111-111111111111"
	hiringApplicant = "22222222-2222-4222-8222-222222222222"
	otherApplicant  = "44444444-4444-4444-8444-444444444444"
)

var (
	hiringAt      = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	hiringCreated = hiringAt.Add(-time.Hour)
)

func statusAt(s domain.Status) domain.ApplicationStatus {
	return domain.ApplicationStatus{ApplicationID: hiringAppID, ApplicantID: hiringApplicant,
		EmployerID: "55555555-5555-4555-8555-555555555555", Status: s, CreatedAt: hiringCreated, UpdatedAt: hiringCreated}
}

func hiring(kind domain.HiringKind, applicant string) domain.HiringEvent {
	return domain.HiringEvent{EventID: "e", ApplicationID: hiringAppID, ApplicantID: applicant, Kind: kind, OccurredAt: hiringAt}
}

// Every hiring event against every stored status: the resulting steps, or the
// error, follow the JT-81 rules and Section 6.
func TestApplyHiring_EveryKindAndStatus(t *testing.T) {
	S := domain.StatusSubmitted
	UR := domain.StatusUnderReview
	SL := domain.StatusShortlisted
	IV := domain.StatusInterview
	OF := domain.StatusOffered
	want := map[domain.HiringKind]map[domain.Status][]domain.Status{
		domain.HiringReviewShortlisted:  {S: {UR, SL}, UR: {SL}},
		domain.HiringReviewRejected:     {S: {domain.StatusRejected}, UR: {domain.StatusRejected}, SL: {domain.StatusRejected}, IV: {domain.StatusRejected}},
		domain.HiringInterviewScheduled: {SL: {IV}, IV: {}},
		domain.HiringOfferMade:          {IV: {OF}},
		domain.HiringOfferAccepted:      {OF: {domain.StatusAccepted}},
		domain.HiringOfferDeclined:      {OF: {domain.StatusDeclined}},
	}
	for kind, byStatus := range want {
		for _, from := range domain.AllStatuses {
			steps, allowed := byStatus[from]
			t.Run(from.String()+"/"+kindName(kind), func(t *testing.T) {
				applicant := ""
				if kind >= domain.HiringOfferMade {
					applicant = hiringApplicant
				}
				got, entries, err := statusAt(from).ApplyHiring(hiring(kind, applicant))
				if !allowed {
					require.ErrorIs(t, err, domain.ErrInvalidTransition)
					assert.Nil(t, entries)
					return
				}
				require.NoError(t, err)
				require.Len(t, entries, len(steps))
				prev := from
				for i, to := range steps {
					assert.Equal(t, domain.HistoryEntry{ApplicationID: hiringAppID, PreviousStatus: prev, NewStatus: to, OccurredAt: hiringAt}, entries[i])
					prev = to
				}
				assert.Equal(t, prev, got.Status)
				if len(steps) == 0 {
					assert.Equal(t, hiringCreated, got.UpdatedAt, "a further interview changes nothing")
				} else {
					assert.Equal(t, hiringAt, got.UpdatedAt)
				}
			})
		}
	}
}

func kindName(k domain.HiringKind) string {
	return map[domain.HiringKind]string{
		domain.HiringReviewShortlisted: "shortlisted", domain.HiringReviewRejected: "rejected",
		domain.HiringInterviewScheduled: "interview", domain.HiringOfferMade: "offer_made",
		domain.HiringOfferAccepted: "offer_accepted", domain.HiringOfferDeclined: "offer_declined",
	}[k]
}

func TestApplyHiring_Conflicts(t *testing.T) {
	for _, kind := range []domain.HiringKind{domain.HiringOfferMade, domain.HiringOfferAccepted, domain.HiringOfferDeclined} {
		t.Run("offer for another applicant/"+kindName(kind), func(t *testing.T) {
			_, entries, err := statusAt(domain.StatusOffered).ApplyHiring(hiring(kind, otherApplicant))
			require.ErrorIs(t, err, domain.ErrConflictingEvent)
			assert.Nil(t, entries)
		})
	}
	t.Run("unknown kind", func(t *testing.T) {
		_, _, err := statusAt(domain.StatusSubmitted).ApplyHiring(hiring(domain.HiringUnspecified, ""))
		require.ErrorIs(t, err, domain.ErrConflictingEvent)
	})
	t.Run("missing time", func(t *testing.T) {
		e := hiring(domain.HiringReviewRejected, "")
		e.OccurredAt = time.Time{}
		_, _, err := statusAt(domain.StatusSubmitted).ApplyHiring(e)
		require.ErrorIs(t, err, domain.ErrInvalidTime)
	})
}

// The full hiring path from SUBMITTED to ACCEPTED, as the five events produce it.
func TestApplyHiring_FullPath(t *testing.T) {
	a := statusAt(domain.StatusSubmitted)
	var all []domain.Status
	for _, e := range []domain.HiringEvent{
		hiring(domain.HiringReviewShortlisted, ""), hiring(domain.HiringInterviewScheduled, ""),
		hiring(domain.HiringInterviewScheduled, ""), hiring(domain.HiringOfferMade, hiringApplicant),
		hiring(domain.HiringOfferAccepted, hiringApplicant),
	} {
		next, entries, err := a.ApplyHiring(e)
		require.NoError(t, err)
		for _, entry := range entries {
			all = append(all, entry.NewStatus)
		}
		a = next
	}
	assert.Equal(t, []domain.Status{domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview,
		domain.StatusOffered, domain.StatusAccepted}, all)
	assert.True(t, a.Status.IsFinal())
}
