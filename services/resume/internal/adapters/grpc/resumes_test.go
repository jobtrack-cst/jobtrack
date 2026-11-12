package grpc_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports/mocks"
)

const (
	ownerID  = "11111111-1111-4111-8111-111111111111"
	otherID  = "22222222-2222-4222-8222-222222222222"
	resumeID = "33333333-3333-4333-8333-333333333333"
)

var (
	errDB      = errors.New("pq: secret SQL detail")
	now        = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	pdfContent = append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0xAB}, 100)...)
)

type handlers struct {
	public   *adapter.ResumeHandler
	internal *adapter.ResumeInternalHandler
	repo     *mocks.MockResumeRepository
}

// Handlers are tested with the real app layer and a mocked repository. Any
// repository call a case does not expect fails the test.
func newHandlers(t *testing.T) handlers {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	h := handlers{repo: mocks.NewMockResumeRepository(ctrl)}
	svc := app.NewResumes(h.repo, clock)
	h.public, h.internal = adapter.NewResumeHandler(svc), adapter.NewResumeInternalHandler(svc)
	return h
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asOwner() context.Context { return withCaller(ownerID, "applicant") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func storedResume() domain.Resume {
	return domain.Resume{
		ID: resumeID, ApplicantID: ownerID, FileName: "cv.pdf", ContentType: "application/pdf",
		SizeBytes: int64(len(pdfContent)), ValidationStatus: domain.ValidationStatusPending,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
}

func storedFile() domain.ResumeFile {
	return domain.ResumeFile{Resume: storedResume(), Content: pdfContent}
}

func uploadRequest() *resumev1.UploadResumeRequest {
	return &resumev1.UploadResumeRequest{FileName: "cv.pdf", ContentType: "application/pdf", Content: pdfContent}
}

// publicCalls runs each public RPC once with a valid request.
var publicCalls = map[string]func(*adapter.ResumeHandler, context.Context) error{
	"UploadResume": func(h *adapter.ResumeHandler, c context.Context) error {
		_, err := h.UploadResume(c, uploadRequest())
		return err
	},
	"GetResume": func(h *adapter.ResumeHandler, c context.Context) error {
		_, err := h.GetResume(c, &resumev1.GetResumeRequest{ResumeId: resumeID})
		return err
	},
	"ListMyResumes": func(h *adapter.ResumeHandler, c context.Context) error {
		_, err := h.ListMyResumes(c, &resumev1.ListMyResumesRequest{})
		return err
	},
	"DownloadResume": func(h *adapter.ResumeHandler, c context.Context) error {
		_, err := h.DownloadResume(c, &resumev1.DownloadResumeRequest{ResumeId: resumeID})
		return err
	},
}

func TestResumeHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", ownerID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "applicant"))},
		{"user ID not a UUID", withCaller("abc", "applicant")},
		{"unknown role", withCaller(ownerID, "superuser")},
		{"uppercase role", withCaller(ownerID, "Applicant")},
	} {
		for rpc, call := range publicCalls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				wantCode(t, call(newHandlers(t).public, tt.ctx), codes.Unauthenticated)
			})
		}
	}
}

// Every public RPC is applicant only, and the role is checked before any
// repository call.
func TestResumeHandler_ApplicantOnlyRPCs(t *testing.T) {
	for _, role := range []string{"employer", "admin"} {
		for rpc, call := range publicCalls {
			t.Run(role+"/"+rpc, func(t *testing.T) {
				wantCode(t, call(newHandlers(t).public, withCaller(ownerID, role)), codes.PermissionDenied)
			})
		}
	}
}

func TestResumeHandler_RoleBeforeInput(t *testing.T) {
	h := newHandlers(t)
	_, err := h.public.UploadResume(withCaller(ownerID, "employer"), &resumev1.UploadResumeRequest{})
	wantCode(t, err, codes.PermissionDenied)
}

func TestResumeHandler_UploadResume_MapsRequestAndResponse(t *testing.T) {
	h := newHandlers(t)
	h.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, f domain.ResumeFile, event *eventsv1.EventEnvelope) (domain.Resume, error) {
			assert.Equal(t, ownerID, f.ApplicantID, "owner comes from metadata, never the request")
			assert.Equal(t, "cv.pdf", f.FileName)
			assert.Equal(t, pdfContent, f.Content)
			return f.Resume, nil
		})
	resp, err := h.public.UploadResume(asOwner(), uploadRequest())
	require.NoError(t, err)
	r := resp.GetResume()
	assert.NotEmpty(t, r.GetResumeId())
	assert.Equal(t, ownerID, r.GetApplicantId())
	assert.Equal(t, "cv.pdf", r.GetFileName())
	assert.Equal(t, "application/pdf", r.GetContentType())
	assert.Equal(t, int64(len(pdfContent)), r.GetSizeBytes())
	assert.Equal(t, resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_PENDING, r.GetValidationStatus())
	assert.Equal(t, now, r.GetTimestamps().GetCreatedAt().AsTime())
	assert.Equal(t, now, r.GetTimestamps().GetUpdatedAt().AsTime())
}

func TestResumeHandler_UploadResume_InvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(*resumev1.UploadResumeRequest)
	}{
		{"content 2,000,001 bytes", func(r *resumev1.UploadResumeRequest) {
			r.Content = append([]byte("%PDF-"), make([]byte, 1_999_996)...)
		}},
		{"empty content", func(r *resumev1.UploadResumeRequest) { r.Content = nil }},
		{"not a pdf", func(r *resumev1.UploadResumeRequest) { r.Content = []byte("hello") }},
		{"wrong content type", func(r *resumev1.UploadResumeRequest) { r.ContentType = "text/plain" }},
		{"missing file name", func(r *resumev1.UploadResumeRequest) { r.FileName = "" }},
		{"file name with path", func(r *resumev1.UploadResumeRequest) { r.FileName = "../cv.pdf" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := uploadRequest()
			tt.edit(req)
			_, err := newHandlers(t).public.UploadResume(asOwner(), req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
	t.Run("nil request", func(t *testing.T) {
		_, err := newHandlers(t).public.UploadResume(asOwner(), nil)
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestResumeHandler_GetResume(t *testing.T) {
	t.Run("owner gets every field, including the validation result", func(t *testing.T) {
		h := newHandlers(t)
		stored := storedResume()
		stored.ValidationStatus, stored.RejectionReason = domain.ValidationStatusRejected, "not a valid PDF"
		h.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(stored, nil)
		resp, err := h.public.GetResume(asOwner(), &resumev1.GetResumeRequest{ResumeId: resumeID})
		require.NoError(t, err)
		assert.Equal(t, &resumev1.Resume{
			ResumeId: resumeID, ApplicantId: ownerID, FileName: "cv.pdf", ContentType: "application/pdf",
			SizeBytes:        int64(len(pdfContent)),
			ValidationStatus: resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED,
			RejectionReason:  "not a valid PDF",
			Timestamps: &commonv1.ResourceTimestamps{
				CreatedAt: resp.GetResume().GetTimestamps().GetCreatedAt(),
				UpdatedAt: resp.GetResume().GetTimestamps().GetUpdatedAt(),
			},
		}, resp.GetResume())
		assert.Equal(t, now.Add(-time.Hour), resp.GetResume().GetTimestamps().GetCreatedAt().AsTime())
	})
	t.Run("validated status", func(t *testing.T) {
		h := newHandlers(t)
		stored := storedResume()
		stored.ValidationStatus = domain.ValidationStatusValidated
		h.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(stored, nil)
		resp, err := h.public.GetResume(asOwner(), &resumev1.GetResumeRequest{ResumeId: resumeID})
		require.NoError(t, err)
		assert.Equal(t, resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED, resp.GetResume().GetValidationStatus())
	})
	t.Run("other applicant is permission denied", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(storedResume(), nil)
		_, err := h.public.GetResume(withCaller(otherID, "applicant"), &resumev1.GetResumeRequest{ResumeId: resumeID})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("unknown resume is not found", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(domain.Resume{}, domain.ErrResumeNotFound)
		_, err := h.public.GetResume(asOwner(), &resumev1.GetResumeRequest{ResumeId: resumeID})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("invalid id", func(t *testing.T) {
		_, err := newHandlers(t).public.GetResume(asOwner(), &resumev1.GetResumeRequest{ResumeId: "abc"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestResumeHandler_ListMyResumes(t *testing.T) {
	t.Run("maps pagination both ways", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().ListByApplicant(gomock.Any(), ownerID, domain.Page{Limit: 1, Offset: 1}).
			Return([]domain.Resume{storedResume()}, 3, nil)
		resp, err := h.public.ListMyResumes(asOwner(), &resumev1.ListMyResumesRequest{
			Pagination: &commonv1.PaginationRequest{PageSize: 1, PageToken: "1"},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetResumes(), 1)
		assert.Equal(t, resumeID, resp.GetResumes()[0].GetResumeId())
		assert.Equal(t, int64(3), resp.GetPagination().GetTotalCount())
		assert.Equal(t, "2", resp.GetPagination().GetNextPageToken())
	})
	t.Run("nil request uses the defaults", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().ListByApplicant(gomock.Any(), ownerID, domain.Page{Limit: 20}).Return([]domain.Resume{}, 0, nil)
		resp, err := h.public.ListMyResumes(asOwner(), nil)
		require.NoError(t, err)
		assert.Empty(t, resp.GetResumes())
		assert.Equal(t, "", resp.GetPagination().GetNextPageToken())
	})
	for _, p := range []*commonv1.PaginationRequest{{PageSize: -1}, {PageToken: "abc"}} {
		t.Run("invalid pagination", func(t *testing.T) {
			_, err := newHandlers(t).public.ListMyResumes(asOwner(), &resumev1.ListMyResumesRequest{Pagination: p})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestResumeHandler_DownloadResume(t *testing.T) {
	t.Run("owner downloads the content", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
		resp, err := h.public.DownloadResume(asOwner(), &resumev1.DownloadResumeRequest{ResumeId: resumeID})
		require.NoError(t, err)
		assert.Equal(t, "cv.pdf", resp.GetFileName())
		assert.Equal(t, "application/pdf", resp.GetContentType())
		assert.Equal(t, pdfContent, resp.GetContent())
	})
	t.Run("other applicant is permission denied", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
		_, err := h.public.DownloadResume(withCaller(otherID, "applicant"), &resumev1.DownloadResumeRequest{ResumeId: resumeID})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("unknown resume is not found", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
		_, err := h.public.DownloadResume(asOwner(), &resumev1.DownloadResumeRequest{ResumeId: resumeID})
		wantCode(t, err, codes.NotFound)
	})
}

func TestResumeInternalHandler_GetResumeFile(t *testing.T) {
	t.Run("returns the file without a user identity", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(storedFile(), nil)
		resp, err := h.internal.GetResumeFile(context.Background(), &resumev1.GetResumeFileRequest{ResumeId: resumeID})
		require.NoError(t, err)
		assert.Equal(t, resumeID, resp.GetResumeId())
		assert.Equal(t, ownerID, resp.GetApplicantId())
		assert.Equal(t, "cv.pdf", resp.GetFileName())
		assert.Equal(t, "application/pdf", resp.GetContentType())
		assert.Equal(t, pdfContent, resp.GetContent())
	})
	t.Run("invalid id", func(t *testing.T) {
		_, err := newHandlers(t).internal.GetResumeFile(context.Background(), &resumev1.GetResumeFileRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("unknown resume is not found", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().GetFile(gomock.Any(), resumeID).Return(domain.ResumeFile{}, domain.ErrResumeNotFound)
		_, err := h.internal.GetResumeFile(context.Background(), &resumev1.GetResumeFileRequest{ResumeId: resumeID})
		wantCode(t, err, codes.NotFound)
	})
}

func TestResumeHandler_ErrorsDoNotLeak(t *testing.T) {
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
			h.repo.EXPECT().GetByID(gomock.Any(), resumeID).Return(domain.Resume{}, tt.repoErr)
			_, err := h.public.GetResume(asOwner(), &resumev1.GetResumeRequest{ResumeId: resumeID})
			wantCode(t, err, tt.wantCode)
			assert.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}
