package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

const (
	partApplicationID = "aaaaaaaa-1111-4111-8111-111111111111"
	partApplicantID   = "bbbbbbbb-2222-4222-8222-222222222222"
	partEmployerID    = "cccccccc-3333-4333-8333-333333333333"
)

func TestNormalizeApplicationID(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
		wantErr        error
	}{
		{"lowercase", partApplicationID, partApplicationID, nil},
		{"uppercase is lowercased", strings.ToUpper(partApplicationID), partApplicationID, nil},
		{"empty", "", "", domain.ErrInvalidApplicationID},
		{"not a UUID", "application-1", "", domain.ErrInvalidApplicationID},
		{"one character short", partApplicationID[:35], "", domain.ErrInvalidApplicationID},
		{"one character long", partApplicationID + "a", "", domain.ErrInvalidApplicationID},
		{"surrounding spaces", " " + partApplicationID + " ", "", domain.ErrInvalidApplicationID},
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

func TestIsUUID(t *testing.T) {
	assert.True(t, domain.IsUUID(partApplicationID))
	assert.True(t, domain.IsUUID(strings.ToUpper(partApplicationID)))
	assert.False(t, domain.IsUUID(""))
	assert.False(t, domain.IsUUID("resume"))
}

func TestNewApplicationParticipant(t *testing.T) {
	want := domain.ApplicationParticipant{ApplicationID: partApplicationID, ApplicantID: partApplicantID, EmployerID: partEmployerID}
	t.Run("valid IDs are stored lowercase", func(t *testing.T) {
		got, err := domain.NewApplicationParticipant(strings.ToUpper(partApplicationID), strings.ToUpper(partApplicantID), strings.ToUpper(partEmployerID))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	for i, name := range []string{"application_id", "applicant_id", "employer_id"} {
		t.Run(name+" not a UUID", func(t *testing.T) {
			ids := []string{partApplicationID, partApplicantID, partEmployerID}
			ids[i] = "not-a-uuid"
			_, err := domain.NewApplicationParticipant(ids[0], ids[1], ids[2])
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
		t.Run("role "+tt.role.String(), func(t *testing.T) {
			require.ErrorIs(t, domain.Caller{UserID: partEmployerID, Role: tt.role}.RequireEmployer(), tt.wantErr)
		})
	}
}

func TestApplicationParticipant_CheckEmployer(t *testing.T) {
	row := domain.ApplicationParticipant{ApplicationID: partApplicationID, ApplicantID: partApplicantID, EmployerID: partEmployerID}
	require.NoError(t, row.CheckEmployer(domain.Caller{UserID: partEmployerID, Role: domain.RoleEmployer}))
	require.ErrorIs(t, row.CheckEmployer(domain.Caller{UserID: partApplicantID, Role: domain.RoleEmployer}), domain.ErrNotApplicationEmployer)
}
