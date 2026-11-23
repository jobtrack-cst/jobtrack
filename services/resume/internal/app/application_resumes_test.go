package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
)

const (
	applicationID   = "aaaaaaaa-1111-4222-8333-444455556666"
	owningEmployer  = "cccccccc-1111-4222-8333-444455556666"
	anotherEmployer = "eeeeeeee-1111-4222-8333-444455556666"
)

type applicationFixture struct {
	projection *mocks.MockApplicationResumeRepository
	repo       *mocks.MockResumeRepository
	svc        *app.ApplicationResumes
}

func newApplicationFixture(t *testing.T) applicationFixture {
	ctrl := gomock.NewController(t)
	f := applicationFixture{projection: mocks.NewMockApplicationResumeRepository(ctrl), repo: mocks.NewMockResumeRepository(ctrl)}
	f.svc = app.NewApplicationResumes(f.projection, f.repo)
	return f
}

func storedApplication() domain.ApplicationResume {
	return domain.ApplicationResume{ApplicationID: applicationID, ApplicantID: ownerID, EmployerID: owningEmployer, ResumeID: resumeID}
}

// One case per row of the JT-114 Section 3 rules table. gomock fails any
// repository call a case does not expect, which proves the check order.
func TestApplicationResumes_Download(t *testing.T) {
	asEmployer := domain.Caller{UserID: owningEmployer, Role: domain.RoleEmployer}
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		setup   func(applicationFixture)
		wantErr error
	}{
		// Row 1: role not employer, checked before input (the ID is invalid too).
		{name: "applicant denied before input", caller: owner, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied before input", caller: admin, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		{name: "missing role denied before input", caller: domain.Caller{UserID: owningEmployer}, id: "bad", wantErr: domain.ErrEmployerRoleRequired},
		// Row 2: application_id not a UUID.
		{name: "empty application_id", caller: asEmployer, id: "", wantErr: domain.ErrInvalidApplicationID},
		{name: "application_id not a UUID", caller: asEmployer, id: "application-1", wantErr: domain.ErrInvalidApplicationID},
		// Row 3: unknown or not processed yet.
		{
			name: "unknown or unprocessed application", caller: asEmployer, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationResume{}, domain.ErrApplicationNotFound)
			},
			wantErr: domain.ErrApplicationNotFound,
		},
		// Row 4: caller is not the stored employer_id.
		{
			name: "another employer denied before the file is read", caller: domain.Caller{UserID: anotherEmployer, Role: domain.RoleEmployer}, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
			},
			wantErr: domain.ErrNotApplicationEmployer,
		},
		// Row 5: resume row no longer exists.
		{
			name: "resume no longer exists", caller: asEmployer, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
			},
			wantErr: domain.ErrResumeNotFound,
		},
		// Unexpected failures pass through for the adapter to map to INTERNAL.
		{
			name: "projection read fails", caller: asEmployer, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationResume{}, errDB)
			},
			wantErr: errDB,
		},
		{
			name: "file read fails", caller: asEmployer, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, errDB)
			},
			wantErr: errDB,
		},
		// Row 6: otherwise, the stored resume.
		{
			name: "owning employer downloads the submitted resume", caller: asEmployer, id: applicationID,
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
			},
		},
		{
			name: "uppercase application_id is normalized", caller: asEmployer, id: "AAAAAAAA-1111-4222-8333-444455556666",
			setup: func(f applicationFixture) {
				f.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newApplicationFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.svc.Download(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got.Content, "no content is returned on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, pdfContent, got.Content)
			assert.Equal(t, "cv.pdf", got.FileName)
			assert.Equal(t, "application/pdf", got.ContentType)
		})
	}
}
