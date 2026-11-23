package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/ports"
)

// ApplicationResumes lets the owning employer download the resume submitted
// with an application (JT-114 Part B). Access is decided only by the local
// application.submitted projection, never by request input.
type ApplicationResumes struct {
	projection ports.ApplicationResumeRepository
	resumes    ports.ResumeRepository
}

func NewApplicationResumes(projection ports.ApplicationResumeRepository, resumes ports.ResumeRepository) *ApplicationResumes {
	return &ApplicationResumes{projection: projection, resumes: resumes}
}

// Download applies the JT-114 rules in order: employer role (before input),
// UUID input, known application, owning employer, existing resume. Access
// stays after a withdrawal, so the employer keeps the record of who applied.
func (a *ApplicationResumes) Download(ctx context.Context, caller domain.Caller, rawApplicationID string) (domain.ResumeFile, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.ResumeFile{}, err
	}
	applicationID, err := domain.NormalizeApplicationID(rawApplicationID)
	if err != nil {
		return domain.ResumeFile{}, err
	}
	row, err := a.projection.GetByApplicationID(ctx, applicationID)
	if err != nil {
		return domain.ResumeFile{}, err
	}
	if err := row.CheckEmployer(caller); err != nil {
		return domain.ResumeFile{}, err
	}
	return a.resumes.GetFile(ctx, row.ResumeID)
}
