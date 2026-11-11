package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// Resumes holds the resume use cases. The gRPC adapter builds the Caller
// from pkg/authctx and translates domain errors to gRPC codes.
// Publishing resume.uploaded is a separate Sprint 2 task; the place that will
// publish it is marked below.
type Resumes struct {
	repo  ports.ResumeRepository
	clock ports.Clock
}

func NewResumes(repo ports.ResumeRepository, clock ports.Clock) *Resumes {
	return &Resumes{repo: repo, clock: clock}
}

// ResumePage is one page of a list result.
type ResumePage struct {
	Resumes       []domain.Resume
	TotalCount    int
	NextPageToken string
}

// Upload stores a PENDING resume owned by the caller. The role is checked
// before the input, so non-applicants learn nothing else.
func (r *Resumes) Upload(ctx context.Context, caller domain.Caller, in domain.UploadInput) (domain.Resume, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Resume{}, err
	}
	upload, err := domain.NewUpload(in)
	if err != nil {
		return domain.Resume{}, err
	}
	now := r.clock.Now().UTC()
	stored, err := r.repo.Create(ctx, domain.ResumeFile{
		Resume: domain.Resume{
			ID: uuid.NewString(), ApplicantID: caller.UserID,
			FileName: upload.FileName, ContentType: upload.ContentType,
			SizeBytes:        int64(len(upload.Content)),
			ValidationStatus: domain.ValidationStatusPending,
			CreatedAt:        now, UpdatedAt: now,
		},
		Content: upload.Content,
	})
	if err != nil {
		return domain.Resume{}, err
	}
	// Sprint 2: publish resume.uploaded here, after the resume is stored.
	return stored, nil
}

// Get returns the metadata of the caller's own resume. application calls this
// with the applicant's forwarded identity to check ownership and VALIDATED status.
func (r *Resumes) Get(ctx context.Context, caller domain.Caller, rawResumeID string) (domain.Resume, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Resume{}, err
	}
	resumeID, err := domain.NormalizeResumeID(rawResumeID)
	if err != nil {
		return domain.Resume{}, err
	}
	resume, err := r.repo.GetByID(ctx, resumeID)
	if err != nil {
		return domain.Resume{}, err
	}
	if err := resume.CheckOwner(caller); err != nil {
		return domain.Resume{}, err
	}
	return resume, nil
}

// ListMine lists only the caller's resumes, newest first, without content.
func (r *Resumes) ListMine(ctx context.Context, caller domain.Caller, pageSize int32, pageToken string) (ResumePage, error) {
	if err := caller.RequireApplicant(); err != nil {
		return ResumePage{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return ResumePage{}, err
	}
	resumes, total, err := r.repo.ListByApplicant(ctx, caller.UserID, page)
	if err != nil {
		return ResumePage{}, err
	}
	return ResumePage{Resumes: resumes, TotalCount: total, NextPageToken: page.NextPageToken(total)}, nil
}

// Download returns the caller's own resume with its content.
func (r *Resumes) Download(ctx context.Context, caller domain.Caller, rawResumeID string) (domain.ResumeFile, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.ResumeFile{}, err
	}
	resumeID, err := domain.NormalizeResumeID(rawResumeID)
	if err != nil {
		return domain.ResumeFile{}, err
	}
	file, err := r.repo.GetFile(ctx, resumeID)
	if err != nil {
		return domain.ResumeFile{}, err
	}
	if err := file.CheckOwner(caller); err != nil {
		return domain.ResumeFile{}, err
	}
	return file, nil
}

// GetFileInternal serves ResumeInternalService.GetResumeFile. There is no user
// identity: the JT-44 interceptor has already allowed only resume-validator.
func (r *Resumes) GetFileInternal(ctx context.Context, rawResumeID string) (domain.ResumeFile, error) {
	resumeID, err := domain.NormalizeResumeID(rawResumeID)
	if err != nil {
		return domain.ResumeFile{}, err
	}
	return r.repo.GetFile(ctx, resumeID)
}
