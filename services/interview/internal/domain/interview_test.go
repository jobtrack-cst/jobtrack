package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	appID      = "aaaaaaaa-1111-4222-8333-444455556666"
	employerID = "eeeeeeee-1111-4222-8333-444455556666"
)

var now = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func TestNewSchedule_TimeWindow(t *testing.T) {
	tests := []struct {
		name    string
		at      time.Time
		wantErr error
	}{
		// Strictly after now (acceptance criterion: INVALID_ARGUMENT otherwise).
		{"missing", time.Time{}, domain.ErrScheduledAtMissing},
		{"one hour ago", now.Add(-time.Hour), domain.ErrScheduledAtNotFuture},
		{"exactly now", now, domain.ErrScheduledAtNotFuture},
		{"one microsecond ago", now.Add(-time.Microsecond), domain.ErrScheduledAtNotFuture},
		{"one microsecond ahead", now.Add(time.Microsecond), nil},
		{"tomorrow", now.Add(24 * time.Hour), nil},
		// At most 365 days ahead.
		{"exactly 365 days ahead", now.Add(365 * 24 * time.Hour), nil},
		{"365 days and a microsecond ahead", now.Add(365*24*time.Hour + time.Microsecond), domain.ErrScheduledAtTooFar},
		{"two years ahead", now.AddDate(2, 0, 0), domain.ErrScheduledAtTooFar},
		// Sub-microsecond precision is dropped first, as Postgres would.
		{"less than a microsecond ahead rounds to now", now.Add(500 * time.Nanosecond), domain.ErrScheduledAtNotFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewSchedule(domain.ScheduleInput{ApplicationID: appID, ScheduledAt: tt.at}, now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.ValidSchedule{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.at.UTC(), got.ScheduledAt)
		})
	}
}

func TestNewSchedule_NormalizesTime(t *testing.T) {
	local := now.Add(48*time.Hour + 123*time.Nanosecond).In(time.FixedZone("BTT", 6*3600))
	got, err := domain.NewSchedule(domain.ScheduleInput{ApplicationID: appID, ScheduledAt: local}, now)
	require.NoError(t, err)
	assert.Equal(t, time.UTC, got.ScheduledAt.Location())
	assert.Equal(t, now.Add(48*time.Hour), got.ScheduledAt, "UTC, truncated to microseconds")
}

func TestNewSchedule_Details(t *testing.T) {
	future := now.Add(24 * time.Hour)
	tests := []struct {
		name    string
		details string
		want    string
		wantErr error
	}{
		{"empty", "", "", nil},
		{"trimmed", "  Room 3, Block B  ", "Room 3, Block B", nil},
		{"line breaks and tab", "Link: https://meet.example/x\n\tBring CV", "Link: https://meet.example/x\n\tBring CV", nil},
		{"1000 characters", strings.Repeat("a", 1000), strings.Repeat("a", 1000), nil},
		{"1000 multibyte", strings.Repeat("ཀ", 1000), strings.Repeat("ཀ", 1000), nil},
		{"1000 after trimming", " " + strings.Repeat("a", 1000) + "\n", strings.Repeat("a", 1000), nil},
		{"1001 characters", strings.Repeat("a", 1001), "", domain.ErrInvalidDetails},
		{"NUL", "a\x00b", "", domain.ErrInvalidDetails},
		{"control character", "a\x07b", "", domain.ErrInvalidDetails},
		{"invalid UTF-8", "a\xffb", "", domain.ErrInvalidDetails},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewSchedule(domain.ScheduleInput{ApplicationID: appID, ScheduledAt: future, Details: tt.details}, now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Details)
		})
	}
}

func TestNewSchedule_ApplicationID(t *testing.T) {
	future := now.Add(time.Hour)
	got, err := domain.NewSchedule(domain.ScheduleInput{ApplicationID: "AAAAAAAA-1111-4222-8333-444455556666", ScheduledAt: future}, now)
	require.NoError(t, err)
	assert.Equal(t, appID, got.ApplicationID)
	for _, bad := range []string{"", "application-1"} {
		_, err := domain.NewSchedule(domain.ScheduleInput{ApplicationID: bad, ScheduledAt: future}, now)
		require.ErrorIs(t, err, domain.ErrInvalidApplicationID)
	}
	// The ID is checked before the time.
	_, err = domain.NewSchedule(domain.ScheduleInput{ApplicationID: "bad"}, now)
	require.ErrorIs(t, err, domain.ErrInvalidApplicationID)
}

func TestApplicationParticipants_Checks(t *testing.T) {
	p := domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
	tests := []struct {
		name      string
		caller    domain.Caller
		ownerErr  error
		readerErr error
	}{
		{"owning employer", domain.Caller{UserID: employerID, Role: domain.RoleEmployer}, nil, nil},
		{"the applicant", domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}, domain.ErrNotOwningEmployer, nil},
		{"another employer", domain.Caller{UserID: otherID, Role: domain.RoleEmployer}, domain.ErrNotOwningEmployer, domain.ErrNotParticipant},
		{"another applicant", domain.Caller{UserID: otherID, Role: domain.RoleApplicant}, domain.ErrNotOwningEmployer, domain.ErrNotParticipant},
		{"applicant ID with employer role", domain.Caller{UserID: applicantID, Role: domain.RoleEmployer}, domain.ErrNotOwningEmployer, domain.ErrNotParticipant},
		{"employer ID with applicant role", domain.Caller{UserID: employerID, Role: domain.RoleApplicant}, domain.ErrNotOwningEmployer, domain.ErrNotParticipant},
		{"admin", domain.Caller{UserID: employerID, Role: domain.RoleAdmin}, domain.ErrNotOwningEmployer, domain.ErrNotParticipant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ownerErr == nil {
				assert.NoError(t, p.CheckOwningEmployer(tt.caller))
			} else {
				assert.ErrorIs(t, p.CheckOwningEmployer(tt.caller), tt.ownerErr)
			}
			if tt.readerErr == nil {
				assert.NoError(t, p.CheckReader(tt.caller))
			} else {
				assert.ErrorIs(t, p.CheckReader(tt.caller), tt.readerErr)
			}
		})
	}
}
