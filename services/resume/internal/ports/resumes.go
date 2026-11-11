package ports

//go:generate go tool mockgen -destination=mocks/resumes_mock.go -package=mocks . ResumeRepository,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

// ResumeRepository stores resumes and their files. Metadata reads never load
// the content, so listing stays small even with 2 MB files.
type ResumeRepository interface {
	// Create stores a new resume with its content and returns the stored metadata.
	Create(ctx context.Context, file domain.ResumeFile) (domain.Resume, error)
	// GetByID returns metadata only, or domain.ErrResumeNotFound.
	GetByID(ctx context.Context, resumeID string) (domain.Resume, error)
	// GetFile returns metadata and content, or domain.ErrResumeNotFound.
	GetFile(ctx context.Context, resumeID string) (domain.ResumeFile, error)
	// ListByApplicant lists one applicant's resumes, newest first (resume_id
	// breaks ties), without content. It also returns the total count.
	ListByApplicant(ctx context.Context, applicantID string, page domain.Page) ([]domain.Resume, int, error)
}

// Clock makes created_at and updated_at testable.
type Clock interface {
	Now() time.Time
}
