package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/ports"
)

type SavedJobs struct {
	repo ports.Repository
}

func NewSavedJobs(repo ports.Repository) *SavedJobs {
	return &SavedJobs{repo: repo}
}

func (s *SavedJobs) SaveJob(ctx context.Context, caller domain.Caller, jobID string) (domain.SavedJob, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.SavedJob{}, err
	}
	if !domain.IsUUID(jobID) {
		return domain.SavedJob{}, domain.ErrInvalidJobID
	}
	if err := s.repo.SaveJob(ctx, jobID, caller.UserID); err != nil {
		return domain.SavedJob{}, err
	}
	// The repo handles the actual timestamp, so for the response we can just return the IDs.
	return domain.SavedJob{JobID: jobID, ApplicantID: caller.UserID}, nil
}

func (s *SavedJobs) UnsaveJob(ctx context.Context, caller domain.Caller, jobID string) error {
	if err := caller.RequireApplicant(); err != nil {
		return err
	}
	if !domain.IsUUID(jobID) {
		return domain.ErrInvalidJobID
	}
	return s.repo.UnsaveJob(ctx, jobID, caller.UserID)
}

func (s *SavedJobs) ListMySavedJobs(ctx context.Context, caller domain.Caller, pageSize int, pageOffset int) (domain.Page, error) {
	if err := caller.RequireApplicant(); err != nil {
		return domain.Page{}, err
	}
	return s.repo.ListMySavedJobs(ctx, caller.UserID, pageSize, pageOffset)
}
func (s *SavedJobs) ListJobSavers(ctx context.Context, jobID string, pageSize int, pageOffset int) ([]string, int, error) {
	if !domain.IsUUID(jobID) {
		return nil, 0, domain.ErrInvalidJobID
	}
	return s.repo.ListJobSavers(ctx, jobID, pageSize, pageOffset)
}
