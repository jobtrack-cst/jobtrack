package app_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
)

const (
	ownerID  = "3f2b8c1e-1111-4222-8333-444455556666"
	otherID  = "9a8b7c6d-1111-4222-8333-444455556666"
	resumeID = "bbbbbbbb-1111-4222-8333-444455556666"
)

var (
	now            = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	owner          = domain.Caller{UserID: ownerID, Role: domain.RoleApplicant}
	otherApplicant = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	employer       = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	admin          = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	errDB          = errors.New("database down")
	pdfContent     = append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 100)...)
)

type fixture struct {
	repo    *mocks.MockResumeRepository
	resumes *app.Resumes
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now.In(time.FixedZone("UTC+6", 6*60*60))).AnyTimes()
	f := fixture{repo: mocks.NewMockResumeRepository(ctrl)}
	f.resumes = app.NewResumes(f.repo, clock)
	return f
}

func validUpload() domain.UploadInput {
	return domain.UploadInput{FileName: " cv.pdf ", ContentType: "application/pdf", Content: pdfContent}
}

func storedResume() domain.Resume {
	return domain.Resume{
		ID: resumeID, ApplicantID: ownerID, FileName: "cv.pdf", ContentType: "application/pdf",
		SizeBytes: int64(len(pdfContent)), ValidationStatus: domain.ValidationStatusPending,
		CreatedAt: now, UpdatedAt: now,
	}
}

func storedFile() domain.ResumeFile {
	return domain.ResumeFile{Resume: storedResume(), Content: pdfContent}
}

// Expectations not set up in a case fail the test if called, so the error
// cases also prove that nothing is stored or read.
func TestResumes_Upload(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		input   func(*domain.UploadInput)
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "applicant uploads a PENDING resume owned by the caller", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, file domain.ResumeFile, event *eventsv1.EventEnvelope) (domain.Resume, error) {
						_, err := uuid.Parse(file.ID)
						assert.NoError(t, err, "resume_id is a generated UUID")
						assert.Equal(t, ownerID, file.ApplicantID)
						assert.Equal(t, "cv.pdf", file.FileName, "file name is trimmed")
						assert.Equal(t, "application/pdf", file.ContentType)
						assert.Equal(t, int64(len(pdfContent)), file.SizeBytes)
						assert.Equal(t, domain.ValidationStatusPending, file.ValidationStatus)
						assert.Empty(t, file.RejectionReason)
						assert.Equal(t, now, file.CreatedAt)
						assert.Equal(t, now, file.UpdatedAt)
						assert.Equal(t, pdfContent, file.Content)
						_, err = uuid.Parse(event.EventId)
						assert.NoError(t, err)
						assert.NotEqual(t, file.ID, event.EventId)
						assert.Equal(t, "resume.uploaded", event.Type)
						assert.Equal(t, "resume", event.Producer)
						assert.Equal(t, now, event.OccurredAt.AsTime())
						payload := new(eventsv1.ResumeUploaded)
						require.NoError(t, event.Data.UnmarshalTo(payload))
						assert.Equal(t, file.ID, payload.ResumeId)
						assert.Equal(t, file.ApplicantID, payload.ApplicantId)
						return file.Resume, nil
					})
			},
		},
		{name: "empty content", caller: owner, input: func(in *domain.UploadInput) { in.Content = nil }, wantErr: domain.ErrInvalidContentSize},
		{name: "empty filename", caller: owner, input: func(in *domain.UploadInput) { in.FileName = " " }, wantErr: domain.ErrInvalidFileName},
		{name: "employer denied", caller: employer, wantErr: domain.ErrApplicantRoleRequired},
		{name: "admin denied", caller: admin, wantErr: domain.ErrApplicantRoleRequired},
		{
			name: "role checked before input", caller: employer,
			input:   func(in *domain.UploadInput) { in.Content = nil },
			wantErr: domain.ErrApplicantRoleRequired,
		},
		{
			name: "file over 2,000,000 bytes", caller: owner,
			input:   func(in *domain.UploadInput) { in.Content = append([]byte("%PDF-"), make([]byte, 1_999_996)...) },
			wantErr: domain.ErrInvalidContentSize,
		},
		{
			name: "not a pdf", caller: owner,
			input:   func(in *domain.UploadInput) { in.Content = []byte("plain text") },
			wantErr: domain.ErrInvalidPDF,
		},
		{
			name: "wrong content type", caller: owner,
			input:   func(in *domain.UploadInput) { in.ContentType = "image/png" },
			wantErr: domain.ErrInvalidContentType,
		},
		{
			name: "repository error", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.Resume{}, errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			in := validUpload()
			if tt.input != nil {
				tt.input(&in)
			}
			got, err := f.resumes.Upload(context.Background(), tt.caller, in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, ownerID, got.ApplicantID)
			assert.Equal(t, domain.ValidationStatusPending, got.ValidationStatus)
		})
	}
}

func TestResumes_Get(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "owner gets metadata", caller: owner, id: resumeID,
			setup: func(f fixture) { f.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(storedResume(), nil) },
		},
		{
			name: "uppercase id is normalized", caller: owner, id: "BBBBBBBB-1111-4222-8333-444455556666",
			setup: func(f fixture) { f.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(storedResume(), nil) },
		},
		{
			name: "other applicant denied", caller: otherApplicant, id: resumeID,
			setup:   func(f fixture) { f.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(storedResume(), nil) },
			wantErr: domain.ErrNotResumeOwner,
		},
		{name: "employer denied before lookup", caller: employer, id: resumeID, wantErr: domain.ErrApplicantRoleRequired},
		{name: "admin denied before lookup", caller: admin, id: resumeID, wantErr: domain.ErrApplicantRoleRequired},
		{name: "invalid id", caller: owner, id: "resume-1", wantErr: domain.ErrInvalidResumeID},
		{
			name: "unknown resume", caller: owner, id: resumeID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(domain.Resume{}, domain.ErrResumeNotFound)
			},
			wantErr: domain.ErrResumeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.resumes.Get(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Resume{}, got, "nothing is returned on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, storedResume(), got)
		})
	}
}

func TestResumes_ListMine(t *testing.T) {
	tests := []struct {
		name      string
		caller    domain.Caller
		size      int32
		token     string
		setup     func(fixture)
		wantTotal int
		wantNext  string
		wantErr   error
	}{
		{
			name: "lists only the caller's resumes with a next page", caller: owner, size: 1,
			setup: func(f fixture) {
				f.repo.EXPECT().ListByApplicant(gomock.Any(), ownerID, domain.Page{Limit: 1, Offset: 0}).
					Return([]domain.Resume{storedResume()}, 2, nil)
			},
			wantTotal: 2, wantNext: "1",
		},
		{
			name: "default page size and last page", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().ListByApplicant(gomock.Any(), ownerID, domain.Page{Limit: 20, Offset: 0}).
					Return([]domain.Resume{storedResume()}, 1, nil)
			},
			wantTotal: 1, wantNext: "",
		},
		{name: "employer denied", caller: employer, wantErr: domain.ErrApplicantRoleRequired},
		{name: "admin denied", caller: admin, wantErr: domain.ErrApplicantRoleRequired},
		{name: "negative page size", caller: owner, size: -1, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: owner, token: "abc", wantErr: domain.ErrInvalidPageToken},
		{
			name: "repository error", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().ListByApplicant(gomock.Any(), ownerID, gomock.Any()).Return(nil, 0, errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.resumes.ListMine(context.Background(), tt.caller, tt.size, tt.token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, got.TotalCount)
			assert.Equal(t, tt.wantNext, got.NextPageToken)
			assert.Len(t, got.Resumes, 1)
		})
	}
}

func TestResumes_Download(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "owner downloads content", caller: owner, id: resumeID,
			setup: func(f fixture) { f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil) },
		},
		{
			name: "other applicant denied", caller: otherApplicant, id: resumeID,
			setup:   func(f fixture) { f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil) },
			wantErr: domain.ErrNotResumeOwner,
		},
		{name: "employer denied before lookup", caller: employer, id: resumeID, wantErr: domain.ErrApplicantRoleRequired},
		{name: "admin denied before lookup", caller: admin, id: resumeID, wantErr: domain.ErrApplicantRoleRequired},
		{name: "invalid id", caller: owner, id: "", wantErr: domain.ErrInvalidResumeID},
		{
			name: "unknown resume", caller: owner, id: resumeID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
			},
			wantErr: domain.ErrResumeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.resumes.Download(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got.Content, "no content is returned on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, pdfContent, got.Content)
			assert.Equal(t, "cv.pdf", got.FileName)
		})
	}
}

func TestResumes_GetFileInternal(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "returns any resume without a user identity", id: resumeID,
			setup: func(f fixture) { f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil) },
		},
		{name: "invalid id", id: "not-a-uuid", wantErr: domain.ErrInvalidResumeID},
		{
			name: "unknown resume", id: resumeID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
			},
			wantErr: domain.ErrResumeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.resumes.GetFileInternal(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, storedFile(), got)
		})
	}
}
