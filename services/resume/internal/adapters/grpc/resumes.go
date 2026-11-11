package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

// ResumeHandler serves the public ResumeService through the gateway, and
// GetResume for application with the applicant's identity forwarded.
type ResumeHandler struct {
	resumev1.UnimplementedResumeServiceServer
	resumes *app.Resumes
}

var _ resumev1.ResumeServiceServer = (*ResumeHandler)(nil)

func NewResumeHandler(resumes *app.Resumes) *ResumeHandler {
	return &ResumeHandler{resumes: resumes}
}

// UploadResume is applicant only. The generated getters are nil-safe, so a
// nil request is an empty one and fails validation.
func (h *ResumeHandler) UploadResume(ctx context.Context, req *resumev1.UploadResumeRequest) (*resumev1.UploadResumeResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resume, err := h.resumes.Upload(ctx, caller, domain.UploadInput{
		FileName: req.GetFileName(), ContentType: req.GetContentType(), Content: req.GetContent(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &resumev1.UploadResumeResponse{Resume: resumeToProto(resume)}, nil
}

func (h *ResumeHandler) GetResume(ctx context.Context, req *resumev1.GetResumeRequest) (*resumev1.GetResumeResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resume, err := h.resumes.Get(ctx, caller, req.GetResumeId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &resumev1.GetResumeResponse{Resume: resumeToProto(resume)}, nil
}

func (h *ResumeHandler) ListMyResumes(ctx context.Context, req *resumev1.ListMyResumesRequest) (*resumev1.ListMyResumesResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	page, err := h.resumes.ListMine(ctx, caller, req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*resumev1.Resume, 0, len(page.Resumes))
	for _, r := range page.Resumes {
		out = append(out, resumeToProto(r))
	}
	return &resumev1.ListMyResumesResponse{
		Resumes:    out,
		Pagination: &commonv1.PaginationResponse{NextPageToken: page.NextPageToken, TotalCount: int64(page.TotalCount)},
	}, nil
}

func (h *ResumeHandler) DownloadResume(ctx context.Context, req *resumev1.DownloadResumeRequest) (*resumev1.DownloadResumeResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	file, err := h.resumes.Download(ctx, caller, req.GetResumeId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &resumev1.DownloadResumeResponse{FileName: file.FileName, ContentType: file.ContentType, Content: file.Content}, nil
}

// ResumeInternalHandler serves ResumeInternalService. The gateway never
// registers it, and the internal server interceptor (pkg/authctx) has already
// checked INTERNAL_TOKEN and that the caller is resume-validator, so there is
// no user role to check here.
type ResumeInternalHandler struct {
	resumev1.UnimplementedResumeInternalServiceServer
	resumes *app.Resumes
}

var _ resumev1.ResumeInternalServiceServer = (*ResumeInternalHandler)(nil)

func NewResumeInternalHandler(resumes *app.Resumes) *ResumeInternalHandler {
	return &ResumeInternalHandler{resumes: resumes}
}

func (h *ResumeInternalHandler) GetResumeFile(ctx context.Context, req *resumev1.GetResumeFileRequest) (*resumev1.GetResumeFileResponse, error) {
	file, err := h.resumes.GetFileInternal(ctx, req.GetResumeId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &resumev1.GetResumeFileResponse{
		ResumeId: file.ID, ApplicantId: file.ApplicantID, FileName: file.FileName,
		ContentType: file.ContentType, Content: file.Content,
	}, nil
}

// callerFrom trusts x-user-id and x-user-role only because the gateway sets
// them and strips any client-sent values (JT-31).
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

// applicantFrom checks the role before the handler reads any input, so a
// non-applicant learns nothing about which inputs are valid.
func applicantFrom(ctx context.Context) (domain.Caller, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireApplicant(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

func statusToProto(s domain.ValidationStatus) resumev1.ResumeValidationStatus {
	switch s {
	case domain.ValidationStatusPending:
		return resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_PENDING
	case domain.ValidationStatusValidated:
		return resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED
	case domain.ValidationStatusRejected:
		return resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED
	default:
		return resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_UNSPECIFIED
	}
}

func resumeToProto(r domain.Resume) *resumev1.Resume {
	return &resumev1.Resume{
		ResumeId:         r.ID,
		ApplicantId:      r.ApplicantID,
		FileName:         r.FileName,
		ContentType:      r.ContentType,
		SizeBytes:        r.SizeBytes,
		ValidationStatus: statusToProto(r.ValidationStatus),
		RejectionReason:  r.RejectionReason,
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(r.CreatedAt),
			UpdatedAt: timestamppb.New(r.UpdatedAt),
		},
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidResumeID), errors.Is(err, domain.ErrInvalidFileName),
		errors.Is(err, domain.ErrInvalidContentType), errors.Is(err, domain.ErrInvalidContentSize),
		errors.Is(err, domain.ErrInvalidPDF), errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrApplicantRoleRequired), errors.Is(err, domain.ErrNotResumeOwner):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrResumeNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
