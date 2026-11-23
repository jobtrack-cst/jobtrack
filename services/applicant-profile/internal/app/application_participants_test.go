package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports/mocks"
)

const (
	applicationID   = "aaaaaaaa-1111-4222-8333-444455556666"
	owningEmployer  = "cccccccc-1111-4222-8333-444455556666"
	anotherEmployer = "eeeeeeee-1111-4222-8333-444455556666"
)

type participantFixture struct {
	participants *mocks.MockApplicationParticipantRepository
	profiles     *mocks.MockProfileRepository
	svc          *app.ApplicationParticipants
}

func newParticipantFixture(t *testing.T) participantFixture {
	ctrl := gomock.NewController(t)
	f := participantFixture{participants: mocks.NewMockApplicationParticipantRepository(ctrl), profiles: mocks.NewMockProfileRepository(ctrl)}
	f.svc = app.NewApplicationParticipants(f.participants, f.profiles)
	return f
}

func storedParticipant() domain.ApplicationParticipant {
	return domain.ApplicationParticipant{ApplicationID: applicationID, ApplicantID: applicantID, EmployerID: owningEmployer}
}

var (
	asOwningEmployer  = domain.Caller{UserID: owningEmployer, Role: domain.RoleEmployer}
	asAnotherEmployer = domain.Caller{UserID: anotherEmployer, Role: domain.RoleEmployer}
)

// accessCase is one row of the JT-114 Section 4 rules table that is decided
// before the profile or photo is read. Both RPCs share these rows.
type accessCase struct {
	name    string
	caller  domain.Caller
	id      string
	lookup  error // nil: no projection read expected
	stored  bool  // the projection read returns storedParticipant()
	wantErr error
}

func accessCases() []accessCase {
	return []accessCase{
		// Row 1: role not employer, checked before input (the ID is invalid too).
		{name: "applicant denied before input", caller: applicant, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied before input", caller: admin, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		{name: "missing role denied before input", caller: domain.Caller{UserID: owningEmployer}, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		// Row 2: application_id not a UUID.
		{name: "empty application_id", caller: asOwningEmployer, id: "", wantErr: domain.ErrInvalidApplicationID},
		{name: "application_id not a UUID", caller: asOwningEmployer, id: "application-1", wantErr: domain.ErrInvalidApplicationID},
		// Row 3: unknown or not processed yet.
		{name: "unknown or unprocessed application", caller: asOwningEmployer, id: applicationID, lookup: domain.ErrApplicationNotFound, wantErr: domain.ErrApplicationNotFound},
		// Row 4: caller is not the stored employer_id.
		{name: "another employer denied before the profile is read", caller: asAnotherEmployer, id: applicationID, stored: true, wantErr: domain.ErrNotApplicationEmployer},
		// Unexpected failures pass through for the adapter to map to INTERNAL.
		{name: "projection read fails", caller: asOwningEmployer, id: applicationID, lookup: errDB, wantErr: errDB},
	}
}

func (c accessCase) setup(f participantFixture) {
	switch {
	case c.lookup != nil:
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationParticipant{}, c.lookup)
	case c.stored:
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
	}
}

// gomock fails any repository call a case does not expect, which proves the
// check order.
func TestApplicationParticipants_Profile_AccessRules(t *testing.T) {
	for _, tt := range accessCases() {
		t.Run(tt.name, func(t *testing.T) {
			f := newParticipantFixture(t)
			tt.setup(f)
			got, err := f.svc.Profile(context.Background(), tt.caller, tt.id)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, domain.Profile{}, got, "no profile is returned on error")
		})
	}
}

func TestApplicationParticipants_Photo_AccessRules(t *testing.T) {
	for _, tt := range accessCases() {
		t.Run(tt.name, func(t *testing.T) {
			f := newParticipantFixture(t)
			tt.setup(f)
			got, err := f.svc.Photo(context.Background(), tt.caller, tt.id)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, got.Content, "no photo is returned on error")
		})
	}
}

func TestApplicationParticipants_Profile(t *testing.T) {
	expectOwned := func(f participantFixture) {
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
	}
	// Row 5: the applicant has no profile.
	t.Run("applicant has no profile", func(t *testing.T) {
		f := newParticipantFixture(t)
		expectOwned(f)
		f.profiles.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{}, domain.ErrProfileNotFound)
		_, err := f.svc.Profile(context.Background(), asOwningEmployer, applicationID)
		require.ErrorIs(t, err, domain.ErrProfileNotFound)
	})
	t.Run("profile read fails", func(t *testing.T) {
		f := newParticipantFixture(t)
		expectOwned(f)
		f.profiles.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{}, errDB)
		_, err := f.svc.Profile(context.Background(), asOwningEmployer, applicationID)
		require.ErrorIs(t, err, errDB)
	})
	// Row 6: otherwise, the current profile of the stored applicant. Only Get
	// is expected, so the photo bytes are never read for a profile.
	for _, id := range []string{applicationID, strings.ToUpper(applicationID)} {
		t.Run("owning employer reads the current profile "+id, func(t *testing.T) {
			f := newParticipantFixture(t)
			expectOwned(f)
			profile := stored()
			profile.HasPhoto = true
			f.profiles.EXPECT().Get(gomock.Any(), applicantID).Return(profile, nil)
			got, err := f.svc.Profile(context.Background(), asOwningEmployer, id)
			require.NoError(t, err)
			assert.Equal(t, profile, got)
		})
	}
}

func TestApplicationParticipants_Photo(t *testing.T) {
	expectOwned := func(f participantFixture) {
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
	}
	// Row 5: no profile or no photo (the repository returns ErrPhotoNotFound
	// for both).
	t.Run("applicant has no photo", func(t *testing.T) {
		f := newParticipantFixture(t)
		expectOwned(f)
		f.profiles.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, domain.ErrPhotoNotFound)
		_, err := f.svc.Photo(context.Background(), asOwningEmployer, applicationID)
		require.ErrorIs(t, err, domain.ErrPhotoNotFound)
	})
	t.Run("photo read fails", func(t *testing.T) {
		f := newParticipantFixture(t)
		expectOwned(f)
		f.profiles.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, errDB)
		_, err := f.svc.Photo(context.Background(), asOwningEmployer, applicationID)
		require.ErrorIs(t, err, errDB)
	})
	// Row 6: otherwise, the current photo.
	t.Run("owning employer reads the current photo", func(t *testing.T) {
		f := newParticipantFixture(t)
		expectOwned(f)
		photo := domain.Photo{ContentType: "image/png", Content: []byte{0x89, 'P', 'N', 'G'}}
		f.profiles.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(photo, nil)
		got, err := f.svc.Photo(context.Background(), asOwningEmployer, applicationID)
		require.NoError(t, err)
		assert.Equal(t, photo, got)
	})
}
