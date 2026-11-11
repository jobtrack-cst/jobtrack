package domain_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	appID      = "aaaaaaaa-1111-4222-8333-444455556666"
	employerID = "eeeeeeee-1111-4222-8333-444455556666"
)

var at = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

type pair struct{ from, to domain.Status }

// allowed is written out by hand from AGENTS.md Section 6, independently of
// the implementation, so the test checks the rules rather than repeating them.
var allowed = map[pair]bool{
	// Main path.
	{domain.StatusSubmitted, domain.StatusUnderReview}:   true,
	{domain.StatusUnderReview, domain.StatusShortlisted}: true,
	{domain.StatusShortlisted, domain.StatusInterview}:   true,
	{domain.StatusInterview, domain.StatusOffered}:       true,
	{domain.StatusOffered, domain.StatusAccepted}:        true,
	{domain.StatusOffered, domain.StatusDeclined}:        true,
	// REJECTED from SUBMITTED, UNDER_REVIEW, SHORTLISTED, INTERVIEW.
	{domain.StatusSubmitted, domain.StatusRejected}:   true,
	{domain.StatusUnderReview, domain.StatusRejected}: true,
	{domain.StatusShortlisted, domain.StatusRejected}: true,
	{domain.StatusInterview, domain.StatusRejected}:   true,
	// WITHDRAWN from any non-final state.
	{domain.StatusSubmitted, domain.StatusWithdrawn}:   true,
	{domain.StatusUnderReview, domain.StatusWithdrawn}: true,
	{domain.StatusShortlisted, domain.StatusWithdrawn}: true,
	{domain.StatusInterview, domain.StatusWithdrawn}:   true,
	{domain.StatusOffered, domain.StatusWithdrawn}:     true,
}

// everyStatus includes UNSPECIFIED, so all 10 x 10 = 100 pairs are checked.
var everyStatus = append([]domain.Status{domain.StatusUnspecified}, domain.AllStatuses...)

func name(s domain.Status) string {
	if s == domain.StatusUnspecified {
		return "unspecified"
	}
	return s.String()
}

func TestCanTransition_EveryPair(t *testing.T) {
	count := 0
	for _, from := range everyStatus {
		for _, to := range everyStatus {
			count++
			want := allowed[pair{from, to}]
			t.Run(fmt.Sprintf("%s to %s", name(from), name(to)), func(t *testing.T) {
				assert.Equal(t, want, domain.CanTransition(from, to))
			})
		}
	}
	assert.Equal(t, 100, count)
	assert.Len(t, allowed, 15, "Section 6 allows exactly 15 transitions")
}

func TestApplicationStatus_Transition_EveryPair(t *testing.T) {
	for _, from := range everyStatus {
		for _, to := range everyStatus {
			t.Run(fmt.Sprintf("%s to %s", name(from), name(to)), func(t *testing.T) {
				before := domain.ApplicationStatus{
					ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID,
					Status: from, CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Hour),
				}
				later := at.In(time.FixedZone("BTT", 6*3600))
				after, entry, err := before.Transition(to, later)
				if !allowed[pair{from, to}] {
					require.ErrorIs(t, err, domain.ErrInvalidTransition)
					assert.Equal(t, domain.ApplicationStatus{}, after)
					assert.Equal(t, domain.HistoryEntry{}, entry)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, to, after.Status)
				assert.Equal(t, at, after.UpdatedAt, "stored in UTC")
				assert.Equal(t, time.UTC, after.UpdatedAt.Location())
				assert.Equal(t, before.CreatedAt, after.CreatedAt, "created_at never changes")
				assert.Equal(t, before.ApplicantID, after.ApplicantID)
				assert.Equal(t, before.EmployerID, after.EmployerID)
				assert.Equal(t, domain.HistoryEntry{ApplicationID: appID, PreviousStatus: from, NewStatus: to, OccurredAt: at}, entry)
			})
		}
	}
}

func TestApplicationStatus_Transition_RequiresTime(t *testing.T) {
	a := domain.ApplicationStatus{ApplicationID: appID, Status: domain.StatusSubmitted}
	_, _, err := a.Transition(domain.StatusUnderReview, time.Time{})
	require.ErrorIs(t, err, domain.ErrInvalidTime)
}

func TestApplicationStatus_FullLifecycles(t *testing.T) {
	paths := map[string][]domain.Status{
		"accepted":                 {domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusOffered, domain.StatusAccepted},
		"declined":                 {domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusOffered, domain.StatusDeclined},
		"rejected after interview": {domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusRejected},
		"withdrawn after offer":    {domain.StatusUnderReview, domain.StatusShortlisted, domain.StatusInterview, domain.StatusOffered, domain.StatusWithdrawn},
	}
	for label, path := range paths {
		t.Run(label, func(t *testing.T) {
			a, first, err := domain.NewSubmitted(appID, applicantID, employerID, at)
			require.NoError(t, err)
			history := []domain.HistoryEntry{first}
			for i, to := range path {
				var entry domain.HistoryEntry
				a, entry, err = a.Transition(to, at.Add(time.Duration(i+1)*time.Minute))
				require.NoError(t, err)
				history = append(history, entry)
			}
			assert.True(t, a.Status.IsFinal())
			require.Len(t, history, len(path)+1)
			for i := 1; i < len(history); i++ {
				assert.Equal(t, history[i-1].NewStatus, history[i].PreviousStatus, "history entries chain")
			}
			for _, to := range domain.AllStatuses {
				_, _, err := a.Transition(to, at.Add(time.Hour))
				assert.ErrorIs(t, err, domain.ErrInvalidTransition, "nothing leaves a final state")
			}
		})
	}
}

func TestNewSubmitted(t *testing.T) {
	t.Run("starts at SUBMITTED with the first history entry", func(t *testing.T) {
		a, entry, err := domain.NewSubmitted("AAAAAAAA-1111-4222-8333-444455556666", applicantID, employerID, at.In(time.FixedZone("BTT", 6*3600)))
		require.NoError(t, err)
		assert.Equal(t, domain.ApplicationStatus{
			ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID,
			Status: domain.StatusSubmitted, CreatedAt: at, UpdatedAt: at,
		}, a)
		assert.Equal(t, domain.HistoryEntry{
			ApplicationID: appID, PreviousStatus: domain.StatusUnspecified, NewStatus: domain.StatusSubmitted, OccurredAt: at,
		}, entry)
	})
	tests := []struct {
		name                    string
		app, applicant, employr string
		at                      time.Time
		wantErr                 error
	}{
		{"bad application id", "x", applicantID, employerID, at, domain.ErrInvalidApplicationID},
		{"bad applicant id", appID, "", employerID, at, domain.ErrInvalidParticipantID},
		{"bad employer id", appID, applicantID, "nope", at, domain.ErrInvalidParticipantID},
		{"missing time", appID, applicantID, employerID, time.Time{}, domain.ErrInvalidTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := domain.NewSubmitted(tt.app, tt.applicant, tt.employr, tt.at)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestApplicationStatus_CheckReader(t *testing.T) {
	a := domain.ApplicationStatus{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID, Status: domain.StatusSubmitted}
	tests := []struct {
		name    string
		caller  domain.Caller
		wantErr error
	}{
		{"the applicant", domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}, nil},
		{"the employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, nil},
		{"another applicant", domain.Caller{UserID: otherID, Role: domain.RoleApplicant}, domain.ErrNotParticipant},
		{"another employer", domain.Caller{UserID: otherID, Role: domain.RoleEmployer}, domain.ErrNotParticipant},
		{"applicant id with employer role", domain.Caller{UserID: applicantID, Role: domain.RoleEmployer}, domain.ErrNotParticipant},
		{"employer id with applicant role", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, domain.ErrNotParticipant},
		{"admin", domain.Caller{UserID: applicantID, Role: domain.RoleAdmin}, domain.ErrNotParticipant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := a.CheckReader(tt.caller)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestStatus_StringParseAndFinal(t *testing.T) {
	stored := map[domain.Status]string{
		domain.StatusSubmitted: "submitted", domain.StatusUnderReview: "under_review",
		domain.StatusShortlisted: "shortlisted", domain.StatusInterview: "interview",
		domain.StatusOffered: "offered", domain.StatusAccepted: "accepted",
		domain.StatusDeclined: "declined", domain.StatusRejected: "rejected", domain.StatusWithdrawn: "withdrawn",
	}
	final := map[domain.Status]bool{
		domain.StatusAccepted: true, domain.StatusDeclined: true, domain.StatusRejected: true, domain.StatusWithdrawn: true,
	}
	require.Len(t, domain.AllStatuses, 9)
	for _, s := range domain.AllStatuses {
		assert.Equal(t, stored[s], s.String())
		got, ok := domain.ParseStatus(stored[s])
		require.True(t, ok)
		assert.Equal(t, s, got)
		assert.Equal(t, final[s], s.IsFinal(), s.String())
	}
	for _, bad := range []string{"", "SUBMITTED", "unknown"} {
		got, ok := domain.ParseStatus(bad)
		assert.False(t, ok)
		assert.Equal(t, domain.StatusUnspecified, got)
	}
	assert.Equal(t, "", domain.StatusUnspecified.String())
	assert.False(t, domain.StatusUnspecified.IsFinal())
}
