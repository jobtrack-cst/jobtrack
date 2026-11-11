package app

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
)

// Jobs holds the job posting use cases. The gRPC adapter builds the Caller
// from pkg/authctx and translates domain errors to gRPC codes.
// Publishing job events is a separate Sprint 2 task; each place that will
// publish one is marked below.
type Jobs struct {
	repo     ports.JobRepository
	verifier ports.EmployerVerifier
	clock    ports.Clock
}

func NewJobs(repo ports.JobRepository, verifier ports.EmployerVerifier, clock ports.Clock) *Jobs {
	return &Jobs{repo: repo, verifier: verifier, clock: clock}
}

// JobPage is one page of a list result.
type JobPage struct {
	Jobs          []domain.Job
	TotalCount    int
	NextPageToken string
}

// Create stores a DRAFT job owned by the caller. The role is checked before
// the input, so non-employers learn nothing else.
func (j *Jobs) Create(ctx context.Context, caller domain.Caller, in domain.JobInput) (domain.Job, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Job{}, err
	}
	now := j.clock.Now()
	fields, err := domain.NewJobFields(in, now)
	if err != nil {
		return domain.Job{}, err
	}
	return j.repo.Create(ctx, domain.Job{
		EmployerID: caller.UserID, JobFields: fields, Status: domain.StatusDraft,
		CreatedAt: now, UpdatedAt: now,
	})
}

// Update replaces the editable fields of the caller's own DRAFT or PUBLISHED job.
func (j *Jobs) Update(ctx context.Context, caller domain.Caller, rawJobID string, in domain.JobInput) (domain.Job, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Job{}, err
	}
	jobID, err := domain.NormalizeJobID(rawJobID)
	if err != nil {
		return domain.Job{}, err
	}
	now := j.clock.Now()
	fields, err := domain.NewJobFields(in, now)
	if err != nil {
		return domain.Job{}, err
	}
	existing, err := j.loadOwned(ctx, caller, jobID)
	if err != nil {
		return domain.Job{}, err
	}
	if err := existing.CheckEditable(); err != nil {
		return domain.Job{}, err
	}
	changed := existing
	changed.JobFields = fields
	changed.UpdatedAt = now
	updated, err := j.repo.UpdateFields(ctx, changed, existing.Status)
	if err != nil {
		return domain.Job{}, err
	}
	// Sprint 2: publish job.updated when updated.Status is PUBLISHED.
	return updated, nil
}

// Publish moves the caller's DRAFT job to PUBLISHED. The employer check is
// last, so no remote call is made for a request that would fail anyway.
func (j *Jobs) Publish(ctx context.Context, caller domain.Caller, rawJobID string) (domain.Job, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Job{}, err
	}
	jobID, err := domain.NormalizeJobID(rawJobID)
	if err != nil {
		return domain.Job{}, err
	}
	existing, err := j.loadOwned(ctx, caller, jobID)
	if err != nil {
		return domain.Job{}, err
	}
	now := j.clock.Now()
	if err := existing.CheckPublishable(now); err != nil {
		return domain.Job{}, err
	}
	verified, err := j.verifier.IsVerified(ctx, existing.EmployerID)
	if err != nil {
		return domain.Job{}, err
	}
	if !verified {
		return domain.Job{}, domain.ErrEmployerNotVerified
	}
	published, err := j.repo.UpdateStatus(ctx, jobID, domain.StatusDraft, domain.StatusPublished, now)
	if err != nil {
		return domain.Job{}, err
	}
	// Sprint 2: publish job.published.
	return published, nil
}

// Close moves the caller's PUBLISHED job to CLOSED.
func (j *Jobs) Close(ctx context.Context, caller domain.Caller, rawJobID string) (domain.Job, error) {
	if err := caller.RequireEmployer(); err != nil {
		return domain.Job{}, err
	}
	jobID, err := domain.NormalizeJobID(rawJobID)
	if err != nil {
		return domain.Job{}, err
	}
	existing, err := j.loadOwned(ctx, caller, jobID)
	if err != nil {
		return domain.Job{}, err
	}
	if err := existing.CheckClosable(); err != nil {
		return domain.Job{}, err
	}
	closed, err := j.repo.UpdateStatus(ctx, jobID, domain.StatusPublished, domain.StatusClosed, j.clock.Now())
	if err != nil {
		return domain.Job{}, err
	}
	// Sprint 2: publish job.closed.
	return closed, nil
}

// Get returns one job to any authenticated role. DRAFT and REMOVED jobs look
// like missing jobs to anyone but the owner and admins.
func (j *Jobs) Get(ctx context.Context, caller domain.Caller, rawJobID string) (domain.Job, error) {
	jobID, err := domain.NormalizeJobID(rawJobID)
	if err != nil {
		return domain.Job{}, err
	}
	job, err := j.repo.GetByID(ctx, jobID)
	if err != nil {
		return domain.Job{}, err
	}
	if !job.VisibleTo(caller) {
		return domain.Job{}, domain.ErrJobNotFound
	}
	return job, nil
}

// ListMine lists the caller's own jobs. StatusUnspecified means all statuses.
func (j *Jobs) ListMine(ctx context.Context, caller domain.Caller, status domain.Status, pageSize int32, pageToken string) (JobPage, error) {
	if err := caller.RequireEmployer(); err != nil {
		return JobPage{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return JobPage{}, err
	}
	jobs, total, err := j.repo.ListByEmployer(ctx, caller.UserID, status, page)
	if err != nil {
		return JobPage{}, err
	}
	return JobPage{Jobs: jobs, TotalCount: total, NextPageToken: page.NextPageToken(total)}, nil
}

// ListClosingBetween serves the internal RPC for deadline-reminder. There is
// no user here; the JT-44 interceptor restricts the caller.
// A zero start or end means the request did not set it.
func (j *Jobs) ListClosingBetween(ctx context.Context, start, end time.Time, pageSize int32, pageToken string) (JobPage, error) {
	window, err := domain.NewClosingWindow(start, end)
	if err != nil {
		return JobPage{}, err
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return JobPage{}, err
	}
	jobs, total, err := j.repo.ListClosingBetween(ctx, window, page)
	if err != nil {
		return JobPage{}, err
	}
	return JobPage{Jobs: jobs, TotalCount: total, NextPageToken: page.NextPageToken(total)}, nil
}

// loadOwned loads a job and allows only its owning employer.
func (j *Jobs) loadOwned(ctx context.Context, caller domain.Caller, jobID string) (domain.Job, error) {
	job, err := j.repo.GetByID(ctx, jobID)
	if err != nil {
		return domain.Job{}, err
	}
	if err := job.CheckOwner(caller); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}
