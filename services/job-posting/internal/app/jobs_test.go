package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	ownerID = "3f2b8c1e-1111-4222-8333-444455556666"
	otherID = "9a8b7c6d-1111-4222-8333-444455556666"
	jobID   = "bbbbbbbb-1111-4222-8333-444455556666"
)

var (
	now           = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	owner         = domain.Caller{UserID: ownerID, Role: domain.RoleEmployer}
	otherEmployer = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	applicant     = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	admin         = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	errDB         = errors.New("database down")
)

type fixture struct {
	repo     *mocks.MockJobRepository
	verifier *mocks.MockEmployerVerifier
	jobs     *app.Jobs
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	f := fixture{repo: mocks.NewMockJobRepository(ctrl), verifier: mocks.NewMockEmployerVerifier(ctrl)}
	f.jobs = app.NewJobs(f.repo, f.verifier, clock)
	return f
}

func validInput() domain.JobInput {
	return domain.JobInput{Title: "Junior Go Developer", Location: "Thimphu", Deadline: now.Add(48 * time.Hour)}
}

func storedJob(status domain.Status) domain.Job {
	return domain.Job{
		ID: jobID, EmployerID: ownerID, Status: status,
		JobFields: domain.JobFields{Title: "Old title", Deadline: now.Add(24 * time.Hour)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
}

func TestJobs_Create(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		input   func(*domain.JobInput)
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "employer creates a draft owned by the caller", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, j domain.Job) (domain.Job, error) {
						assert.Equal(t, ownerID, j.EmployerID)
						assert.Equal(t, domain.StatusDraft, j.Status)
						assert.Equal(t, "Junior Go Developer", j.Title)
						assert.Equal(t, now, j.CreatedAt)
						assert.Equal(t, now, j.UpdatedAt)
						j.ID = jobID
						return j, nil
					})
			},
		},
		{name: "applicant denied", caller: applicant, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, wantErr: domain.ErrEmployerRoleRequired},
		{
			name: "role checked before input", caller: applicant,
			input: func(in *domain.JobInput) { in.Title = "" }, wantErr: domain.ErrEmployerRoleRequired,
		},
		{name: "empty title", caller: owner, input: func(in *domain.JobInput) { in.Title = "" }, wantErr: domain.ErrInvalidTitle},
		{name: "deadline in past", caller: owner, input: func(in *domain.JobInput) { in.Deadline = now.Add(-time.Minute) }, wantErr: domain.ErrInvalidDeadline},
		{
			name: "repository error", caller: owner,
			setup:   func(f fixture) { f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Job{}, errDB) },
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			in := validInput()
			if tt.input != nil {
				tt.input(&in)
			}
			got, err := f.jobs.Create(context.Background(), tt.caller, in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, jobID, got.ID)
		})
	}
}

func TestJobs_Update(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		jobID   string
		input   func(*domain.JobInput)
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "owner edits a draft", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
				f.repo.EXPECT().UpdateFields(gomock.Any(), gomock.Any(), domain.StatusDraft).DoAndReturn(
					func(_ context.Context, j domain.Job, _ domain.Status) (domain.Job, error) {
						assert.Equal(t, "Junior Go Developer", j.Title)
						assert.Equal(t, domain.StatusDraft, j.Status, "update never changes status")
						assert.Equal(t, now.Add(-time.Hour), j.CreatedAt)
						assert.Equal(t, now, j.UpdatedAt)
						return j, nil
					})
			},
		},
		{
			name: "owner edits a published job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
				f.repo.EXPECT().UpdateFields(gomock.Any(), gomock.Any(), domain.StatusPublished).
					Return(storedJob(domain.StatusPublished), nil)
			},
		},
		{name: "applicant denied", caller: applicant, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "role checked before job id", caller: admin, jobID: "bad", wantErr: domain.ErrEmployerRoleRequired},
		{name: "job id not a uuid", caller: owner, jobID: "bad", wantErr: domain.ErrInvalidJobID},
		{name: "invalid input", caller: owner, jobID: jobID, input: func(in *domain.JobInput) { in.Title = "" }, wantErr: domain.ErrInvalidTitle},
		{
			name: "job not found", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, domain.ErrJobNotFound)
			},
			wantErr: domain.ErrJobNotFound,
		},
		{
			name: "other employer denied", caller: otherEmployer, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
			},
			wantErr: domain.ErrNotJobOwner,
		},
		{
			name: "closed job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusClosed), nil)
			},
			wantErr: domain.ErrJobNotEditable,
		},
		{
			name: "removed job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusRemoved), nil)
			},
			wantErr: domain.ErrJobNotEditable,
		},
		{
			name: "status changed concurrently", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
				f.repo.EXPECT().UpdateFields(gomock.Any(), gomock.Any(), domain.StatusPublished).
					Return(domain.Job{}, domain.ErrJobStatusChanged)
			},
			wantErr: domain.ErrJobStatusChanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			in := validInput()
			if tt.input != nil {
				tt.input(&in)
			}
			_, err := f.jobs.Update(context.Background(), tt.caller, tt.jobID, in)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJobs_Publish(t *testing.T) {
	draftPastDeadline := storedJob(domain.StatusDraft)
	draftPastDeadline.Deadline = now.Add(-time.Minute)

	tests := []struct {
		name    string
		caller  domain.Caller
		jobID   string
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "verified owner publishes a draft", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
				f.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(true, nil)
				f.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusDraft, domain.StatusPublished, now).
					Return(storedJob(domain.StatusPublished), nil)
			},
		},
		{name: "applicant denied", caller: applicant, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "job id not a uuid", caller: owner, jobID: "", wantErr: domain.ErrInvalidJobID},
		{
			name: "job not found", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, domain.ErrJobNotFound)
			},
			wantErr: domain.ErrJobNotFound,
		},
		{
			name: "other employer denied without calling employer service", caller: otherEmployer, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
			},
			wantErr: domain.ErrNotJobOwner,
		},
		{
			name: "already published", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
			},
			wantErr: domain.ErrJobNotDraft,
		},
		{
			name: "closed job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusClosed), nil)
			},
			wantErr: domain.ErrJobNotDraft,
		},
		{
			name: "removed job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusRemoved), nil)
			},
			wantErr: domain.ErrJobNotDraft,
		},
		{
			name: "deadline passed", caller: owner, jobID: jobID,
			setup:   func(f fixture) { f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(draftPastDeadline, nil) },
			wantErr: domain.ErrDeadlinePassed,
		},
		{
			name: "employer not verified", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
				f.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(false, nil)
			},
			wantErr: domain.ErrEmployerNotVerified,
		},
		{
			name: "employer service error", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
				f.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(false, errDB)
			},
			wantErr: errDB,
		},
		{
			name: "published concurrently", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
				f.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(true, nil)
				f.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusDraft, domain.StatusPublished, now).
					Return(domain.Job{}, domain.ErrJobStatusChanged)
			},
			wantErr: domain.ErrJobStatusChanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.jobs.Publish(context.Background(), tt.caller, tt.jobID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.StatusPublished, got.Status)
		})
	}
}

func TestJobs_Close(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		jobID   string
		setup   func(fixture)
		wantErr error
	}{
		{
			name: "owner closes a published job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
				f.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusPublished, domain.StatusClosed, now).
					Return(storedJob(domain.StatusClosed), nil)
			},
		},
		{name: "applicant denied", caller: applicant, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, jobID: jobID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "job id not a uuid", caller: owner, jobID: "x", wantErr: domain.ErrInvalidJobID},
		{
			name: "job not found", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, domain.ErrJobNotFound)
			},
			wantErr: domain.ErrJobNotFound,
		},
		{
			name: "other employer denied", caller: otherEmployer, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
			},
			wantErr: domain.ErrNotJobOwner,
		},
		{
			name: "draft job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
			},
			wantErr: domain.ErrJobNotPublished,
		},
		{
			name: "already closed", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusClosed), nil)
			},
			wantErr: domain.ErrJobNotPublished,
		},
		{
			name: "removed job", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusRemoved), nil)
			},
			wantErr: domain.ErrJobNotPublished,
		},
		{
			name: "closed concurrently", caller: owner, jobID: jobID,
			setup: func(f fixture) {
				f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusPublished), nil)
				f.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusPublished, domain.StatusClosed, now).
					Return(domain.Job{}, domain.ErrJobStatusChanged)
			},
			wantErr: domain.ErrJobStatusChanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.jobs.Close(context.Background(), tt.caller, tt.jobID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.StatusClosed, got.Status)
		})
	}
}

func TestJobs_Get(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		status  domain.Status
		wantErr error
	}{
		{"applicant sees published", applicant, domain.StatusPublished, nil},
		{"applicant sees closed", applicant, domain.StatusClosed, nil},
		{"applicant cannot see draft", applicant, domain.StatusDraft, domain.ErrJobNotFound},
		{"applicant cannot see removed", applicant, domain.StatusRemoved, domain.ErrJobNotFound},
		{"other employer cannot see draft", otherEmployer, domain.StatusDraft, domain.ErrJobNotFound},
		{"other employer sees published", otherEmployer, domain.StatusPublished, nil},
		{"owner sees draft", owner, domain.StatusDraft, nil},
		{"owner sees removed", owner, domain.StatusRemoved, nil},
		{"admin sees draft", admin, domain.StatusDraft, nil},
		{"admin sees removed", admin, domain.StatusRemoved, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(tt.status), nil)
			got, err := f.jobs.Get(context.Background(), tt.caller, jobID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, now.Add(24*time.Hour), got.Deadline)
		})
	}

	t.Run("job id not a uuid", func(t *testing.T) {
		_, err := newFixture(t).jobs.Get(context.Background(), applicant, "nope")
		require.ErrorIs(t, err, domain.ErrInvalidJobID)
	})
	t.Run("job not found", func(t *testing.T) {
		f := newFixture(t)
		f.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, domain.ErrJobNotFound)
		_, err := f.jobs.Get(context.Background(), applicant, jobID)
		require.ErrorIs(t, err, domain.ErrJobNotFound)
	})
}

func TestJobs_ListMine(t *testing.T) {
	tests := []struct {
		name      string
		caller    domain.Caller
		status    domain.Status
		size      int32
		token     string
		setup     func(fixture)
		wantNext  string
		wantTotal int
		wantErr   error
	}{
		{
			name: "first page of all statuses", caller: owner, size: 2,
			setup: func(f fixture) {
				f.repo.EXPECT().ListByEmployer(gomock.Any(), ownerID, domain.StatusUnspecified, domain.Page{Limit: 2}).
					Return([]domain.Job{storedJob(domain.StatusDraft), storedJob(domain.StatusPublished)}, 5, nil)
			},
			wantNext: "2", wantTotal: 5,
		},
		{
			name: "last page filtered by status", caller: owner, status: domain.StatusClosed, token: "20",
			setup: func(f fixture) {
				f.repo.EXPECT().ListByEmployer(gomock.Any(), ownerID, domain.StatusClosed, domain.Page{Limit: 20, Offset: 20}).
					Return([]domain.Job{storedJob(domain.StatusClosed)}, 21, nil)
			},
			wantNext: "", wantTotal: 21,
		},
		{name: "applicant denied", caller: applicant, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, wantErr: domain.ErrEmployerRoleRequired},
		{name: "negative page size", caller: owner, size: -1, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: owner, token: "abc", wantErr: domain.ErrInvalidPageToken},
		{
			name: "repository error", caller: owner,
			setup: func(f fixture) {
				f.repo.EXPECT().ListByEmployer(gomock.Any(), ownerID, domain.StatusUnspecified, gomock.Any()).Return(nil, 0, errDB)
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
			got, err := f.jobs.ListMine(context.Background(), tt.caller, tt.status, tt.size, tt.token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNext, got.NextPageToken)
			assert.Equal(t, tt.wantTotal, got.TotalCount)
		})
	}
}

func TestJobs_ListClosingBetween(t *testing.T) {
	start, end := now, now.Add(48*time.Hour)
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		token    string
		setup    func(fixture)
		wantNext string
		wantErr  error
	}{
		{
			name: "lists the window", start: start, end: end,
			setup: func(f fixture) {
				f.repo.EXPECT().ListClosingBetween(gomock.Any(), domain.ClosingWindow{Start: start, End: end}, domain.Page{Limit: 20}).
					Return([]domain.Job{storedJob(domain.StatusPublished)}, 1, nil)
			},
		},
		{
			name: "more pages", start: start, end: end,
			setup: func(f fixture) {
				f.repo.EXPECT().ListClosingBetween(gomock.Any(), gomock.Any(), domain.Page{Limit: 20}).
					Return(make([]domain.Job, 20), 30, nil)
			},
			wantNext: "20",
		},
		{name: "missing start", end: end, wantErr: domain.ErrInvalidTimeRange},
		{name: "missing end", start: start, wantErr: domain.ErrInvalidTimeRange},
		{name: "start after end", start: end, end: start, wantErr: domain.ErrInvalidTimeRange},
		{name: "bad page token", start: start, end: end, token: "-1", wantErr: domain.ErrInvalidPageToken},
		{
			name: "repository error", start: start, end: end,
			setup: func(f fixture) {
				f.repo.EXPECT().ListClosingBetween(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, 0, errDB)
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
			got, err := f.jobs.ListClosingBetween(context.Background(), tt.start, tt.end, 0, tt.token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNext, got.NextPageToken)
		})
	}
}
