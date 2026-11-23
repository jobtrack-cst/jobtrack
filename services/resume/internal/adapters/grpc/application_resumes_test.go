package grpc_test

import (
	"context"
	"testing"

	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

const (
	applicationID   = "44444444-4444-4444-8444-444444444444"
	owningEmployer  = "55555555-5555-4555-8555-555555555555"
	anotherEmployer = "66666666-6666-4666-8666-666666666666"
)

func storedApplication() domain.ApplicationResume {
	return domain.ApplicationResume{ApplicationID: applicationID, ApplicantID: ownerID, EmployerID: owningEmployer, ResumeID: resumeID}
}

func downloadApplicationResume(h handlers, ctx context.Context, id string) (*resumev1.DownloadApplicationResumeResponse, error) {
	return h.public.DownloadApplicationResume(ctx, &resumev1.DownloadApplicationResumeRequest{ApplicationId: id})
}

// One case per row of the JT-114 Section 3 rules table, checked as gRPC codes.
func TestResumeHandler_DownloadApplicationResume_Rules(t *testing.T) {
	asEmployer := withCaller(owningEmployer, "employer")
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		id    string
		setup func(handlers)
		want  codes.Code
	}{
		{name: "applicant is denied before input", ctx: asOwner(), id: "bad", want: codes.PermissionDenied},
		{name: "admin is denied before input", ctx: withCaller(owningEmployer, "admin"), id: "bad", want: codes.PermissionDenied},
		{name: "application_id not a UUID", ctx: asEmployer, id: "bad", want: codes.InvalidArgument},
		{name: "empty request", ctx: asEmployer, id: "", want: codes.InvalidArgument},
		{
			name: "unknown or unprocessed application", ctx: asEmployer, id: applicationID, want: codes.NotFound,
			setup: func(h handlers) {
				h.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationResume{}, domain.ErrApplicationNotFound)
			},
		},
		{
			name: "another employer", ctx: withCaller(anotherEmployer, "employer"), id: applicationID, want: codes.PermissionDenied,
			setup: func(h handlers) {
				h.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
			},
		},
		{
			name: "resume no longer exists", ctx: asEmployer, id: applicationID, want: codes.NotFound,
			setup: func(h handlers) {
				h.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
			},
		},
		{
			name: "owning employer", ctx: asEmployer, id: applicationID, want: codes.OK,
			setup: func(h handlers) {
				h.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedApplication(), nil)
				h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlers(t)
			if tt.setup != nil {
				tt.setup(h)
			}
			resp, err := downloadApplicationResume(h, tt.ctx, tt.id)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				assert.Nil(t, resp)
				return
			}
			assert.Equal(t, "cv.pdf", resp.GetFileName())
			assert.Equal(t, "application/pdf", resp.GetContentType())
			assert.Equal(t, pdfContent, resp.GetContent())
		})
	}
}

func TestResumeHandler_DownloadApplicationResume_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", owningEmployer))},
		{"user ID not a UUID", withCaller("abc", "employer")},
		{"uppercase role", withCaller(owningEmployer, "Employer")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := downloadApplicationResume(newHandlers(t), tt.ctx, applicationID)
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

func TestResumeHandler_DownloadApplicationResume_ErrorsDoNotLeak(t *testing.T) {
	for _, tt := range []struct {
		name     string
		repoErr  error
		wantCode codes.Code
		wantMsg  string
	}{
		{"database error", errDB, codes.Internal, "internal error"},
		{"canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlers(t)
			h.projection.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationResume{}, tt.repoErr)
			_, err := downloadApplicationResume(h, withCaller(owningEmployer, "employer"), applicationID)
			wantCode(t, err, tt.wantCode)
			require.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}
