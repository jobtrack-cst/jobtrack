package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

const (
	jobID       = "33333333-3333-4333-8333-333333333333"
	applicantID = "22222222-2222-4222-8222-222222222222"
)

var deadline = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

func TestPeriod_Is48HoursFromNowInUTC(t *testing.T) {
	thimphu := time.FixedZone("BTT", 6*3600)
	now := time.Date(2026, 11, 15, 22, 0, 0, 123456789, thimphu)
	start, end := domain.Period(now)
	assert.Equal(t, time.Date(2026, 11, 15, 16, 0, 0, 123456000, time.UTC), start)
	assert.Equal(t, start.Add(48*time.Hour), end)
	assert.Equal(t, time.UTC, start.Location())
}

func TestNewReminder(t *testing.T) {
	r, err := domain.NewReminder(strings.ToUpper(jobID), strings.ToUpper(applicantID), deadline.Add(999).In(time.FixedZone("X", 3600)))
	require.NoError(t, err)
	assert.Equal(t, domain.Reminder{JobID: jobID, ApplicantID: applicantID, Deadline: deadline}, r, "IDs lowercase, deadline UTC to the microsecond")

	for name, in := range map[string][3]any{
		"job not a UUID":        {"x", applicantID, deadline},
		"job without dashes":    {strings.ReplaceAll(jobID, "-", ""), applicantID, deadline},
		"applicant not a UUID":  {jobID, "", deadline},
		"applicant with braces": {jobID, "{" + applicantID + "}", deadline},
		"missing deadline":      {jobID, applicantID, time.Time{}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewReminder(in[0].(string), in[1].(string), in[2].(time.Time))
			require.ErrorIs(t, err, domain.ErrInvalidReminder)
		})
	}
}

// The same saver, job and deadline always give the same event_id, so a
// repeated or overlapping run publishes nothing new to consumers.
func TestEventID_StableAndDistinct(t *testing.T) {
	r, err := domain.NewReminder(jobID, applicantID, deadline)
	require.NoError(t, err)
	id := r.EventID()
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(5), parsed.Version())

	again, err := domain.NewReminder(strings.ToUpper(jobID), applicantID, deadline.In(time.FixedZone("BTT", 6*3600)).Add(500))
	require.NoError(t, err)
	assert.Equal(t, id, again.EventID(), "same reminder however the input was written")

	for name, other := range map[string]domain.Reminder{
		"another applicant":  {JobID: jobID, ApplicantID: "44444444-4444-4444-8444-444444444444", Deadline: deadline},
		"another job":        {JobID: "55555555-5555-4555-8555-555555555555", ApplicantID: applicantID, Deadline: deadline},
		"a changed deadline": {JobID: jobID, ApplicantID: applicantID, Deadline: deadline.Add(time.Microsecond)},
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotEqual(t, id, other.EventID())
		})
	}
}
