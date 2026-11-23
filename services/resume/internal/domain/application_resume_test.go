package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

const (
	appID      = "aaaaaaaa-1111-4111-8111-111111111111"
	applicant  = "bbbbbbbb-2222-4222-8222-222222222222"
	employerID = "cccccccc-3333-4333-8333-333333333333"
	resume     = "dddddddd-4444-4444-8444-444444444444"
)

func TestNormalizeApplicationID(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
		wantErr        error
	}{
		{"lowercase", appID, appID, nil},
		{"uppercase is lowercased", "AAAAAAAA-1111-4111-8111-111111111111", appID, nil},
		{"empty", "", "", domain.ErrInvalidApplicationID},
		{"not a UUID", "application-1", "", domain.ErrInvalidApplicationID},
		{"one character short", appID[:35], "", domain.ErrInvalidApplicationID},
		{"surrounding spaces", " " + appID + " ", "", domain.ErrInvalidApplicationID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeApplicationID(tt.in)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeEventID(t *testing.T) {
	got, err := domain.NormalizeEventID("EEEEEEEE-5555-4555-8555-555555555555")
	require.NoError(t, err)
	assert.Equal(t, "eeeeeeee-5555-4555-8555-555555555555", got)
	_, err = domain.NormalizeEventID("event-1")
	require.ErrorIs(t, err, domain.ErrInvalidApplicationEvent)
}

func TestNewApplicationResume(t *testing.T) {
	want := domain.ApplicationResume{ApplicationID: appID, ApplicantID: applicant, EmployerID: employerID, ResumeID: resume}
	t.Run("valid IDs are stored lowercase", func(t *testing.T) {
		got, err := domain.NewApplicationResume(upper(appID), upper(applicant), upper(employerID), upper(resume))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	for i, name := range []string{"application_id", "applicant_id", "employer_id", "resume_id"} {
		t.Run(name+" not a UUID", func(t *testing.T) {
			ids := []string{appID, applicant, employerID, resume}
			ids[i] = "not-a-uuid"
			_, err := domain.NewApplicationResume(ids[0], ids[1], ids[2], ids[3])
			require.ErrorIs(t, err, domain.ErrInvalidApplicationEvent)
		})
	}
}

func TestCaller_RequireEmployer(t *testing.T) {
	for _, tt := range []struct {
		role    domain.Role
		wantErr error
	}{
		{domain.RoleEmployer, nil},
		{domain.RoleApplicant, domain.ErrEmployerRoleRequired},
		{domain.RoleAdmin, domain.ErrEmployerRoleRequired},
		{domain.RoleUnspecified, domain.ErrEmployerRoleRequired},
	} {
		t.Run(tt.role.String(), func(t *testing.T) {
			require.ErrorIs(t, domain.Caller{UserID: employerID, Role: tt.role}.RequireEmployer(), tt.wantErr)
		})
	}
}

func TestApplicationResume_CheckEmployer(t *testing.T) {
	row := domain.ApplicationResume{ApplicationID: appID, ApplicantID: applicant, EmployerID: employerID, ResumeID: resume}
	require.NoError(t, row.CheckEmployer(domain.Caller{UserID: employerID, Role: domain.RoleEmployer}))
	require.ErrorIs(t, row.CheckEmployer(domain.Caller{UserID: applicant, Role: domain.RoleEmployer}), domain.ErrNotApplicationEmployer)
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'f' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
