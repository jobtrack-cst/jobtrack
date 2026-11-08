package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports/mocks"
)

const (
	reporterID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	adminID    = "8c3d4e5f-6a7b-4c8d-ae9f-1a2b3c4d5e6f"
	jobID      = "7b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"
	reportID   = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
)

var (
	// A non-UTC clock shows that stored times are converted to UTC.
	now       = time.Date(2026, 11, 8, 15, 30, 0, 0, time.FixedZone("BTT", 6*60*60))
	created   = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	applicant = domain.Caller{UserID: reporterID, Role: domain.RoleApplicant}
	employer  = domain.Caller{UserID: reporterID, Role: domain.RoleEmployer}
	admin     = domain.Caller{UserID: adminID, Role: domain.RoleAdmin}
	errDB     = errors.New("database unavailable")
)

// newReports returns a service whose mocks fail the test on any call the
// case did not expect, so "no repository call" is checked by default.
func newReports(t *testing.T) (*app.Reports, *mocks.MockReportRepository, *mocks.MockClock) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockReportRepository(ctrl)
	clock := mocks.NewMockClock(ctrl)
	return app.NewReports(repo, clock), repo, clock
}

func storedReport(status domain.ReportStatus) domain.Report {
	return domain.Report{
		ID:         reportID,
		JobID:      jobID,
		ReporterID: reporterID,
		Reason:     "spam",
		Status:     status,
		CreatedAt:  created,
		UpdatedAt:  created,
	}
}

func TestReports_Report(t *testing.T) {
	// echo returns what the service passed to Create, so each case can check it.
	echo := func(_ context.Context, r domain.Report) (domain.Report, error) { return r, nil }

	tests := []struct {
		name       string
		caller     domain.Caller
		jobID      string
		reason     string
		setup      func(repo *mocks.MockReportRepository, clock *mocks.MockClock)
		wantErr    error
		wantJobID  string
		wantReason string
	}{
		{
			name: "applicant can report", caller: applicant, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echo)
			},
			wantJobID: jobID, wantReason: "spam",
		},
		{
			name: "employer can report", caller: employer, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echo)
			},
			wantJobID: jobID, wantReason: "spam",
		},
		{
			name: "admin can report", caller: admin, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echo)
			},
			wantJobID: jobID, wantReason: "spam",
		},
		{
			name: "job ID is lowercased and reason trimmed", caller: applicant,
			jobID: strings.ToUpper(jobID), reason: "  fake company \n",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echo)
			},
			wantJobID: jobID, wantReason: "fake company",
		},
		{
			name: "reason at the maximum", caller: applicant, jobID: jobID, reason: strings.Repeat("a", 1000),
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echo)
			},
			wantJobID: jobID, wantReason: strings.Repeat("a", 1000),
		},
		{name: "job ID not a UUID", caller: applicant, jobID: "job-1", reason: "spam", wantErr: domain.ErrInvalidJobID},
		{name: "empty job ID", caller: applicant, jobID: "", reason: "spam", wantErr: domain.ErrInvalidJobID},
		{name: "empty reason", caller: applicant, jobID: jobID, reason: "", wantErr: domain.ErrInvalidReportReason},
		{name: "only spaces reason", caller: applicant, jobID: jobID, reason: "   ", wantErr: domain.ErrInvalidReportReason},
		{name: "reason one over the maximum", caller: applicant, jobID: jobID, reason: strings.Repeat("a", 1001), wantErr: domain.ErrInvalidReportReason},
		{
			name: "job already removed", caller: applicant, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Report{}, domain.ErrJobAlreadyRemoved)
			},
			wantErr: domain.ErrJobAlreadyRemoved,
		},
		{
			name: "open report already exists", caller: applicant, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Report{}, domain.ErrOpenReportExists)
			},
			wantErr: domain.ErrOpenReportExists,
		},
		{
			name: "repository error is returned", caller: applicant, jobID: jobID, reason: "spam",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Report{}, errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, clock := newReports(t)
			if tt.setup != nil {
				tt.setup(repo, clock)
			}

			got, err := svc.Report(context.Background(), tt.caller, tt.jobID, tt.reason)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Report{}, got)
				return
			}
			require.NoError(t, err)
			_, parseErr := uuid.Parse(got.ID)
			require.NoError(t, parseErr, "report ID must be a UUID")
			assert.Equal(t, tt.wantJobID, got.JobID)
			assert.Equal(t, tt.caller.UserID, got.ReporterID, "reporter comes from the caller")
			assert.Equal(t, tt.wantReason, got.Reason)
			assert.Equal(t, domain.StatusOpen, got.Status)
			assert.Equal(t, now.UTC(), got.CreatedAt)
			assert.Equal(t, now.UTC(), got.UpdatedAt)
		})
	}
}

func TestReports_Report_NewIDEachTime(t *testing.T) {
	svc, repo, clock := newReports(t)
	clock.EXPECT().Now().Return(now).Times(2)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r domain.Report) (domain.Report, error) { return r, nil },
	).Times(2)

	first, err := svc.Report(context.Background(), applicant, jobID, "spam")
	require.NoError(t, err)
	second, err := svc.Report(context.Background(), employer, jobID, "spam")
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
}

func TestReports_List(t *testing.T) {
	items := []domain.Report{storedReport(domain.StatusOpen), storedReport(domain.StatusOpen)}

	tests := []struct {
		name      string
		caller    domain.Caller
		status    int32
		pageSize  int32
		pageToken string
		setup     func(repo *mocks.MockReportRepository)
		wantErr   error
		want      app.ListResult
	}{
		{name: "applicant is denied", caller: applicant, wantErr: domain.ErrAdminRoleRequired},
		{name: "employer is denied", caller: employer, wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the filter", caller: applicant, status: 7, wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the page", caller: employer, pageSize: -1, wantErr: domain.ErrAdminRoleRequired},
		{name: "unknown status filter", caller: admin, status: 4, wantErr: domain.ErrInvalidStatusFilter},
		{name: "negative status filter", caller: admin, status: -1, wantErr: domain.ErrInvalidStatusFilter},
		{name: "negative page size", caller: admin, pageSize: -1, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: admin, pageToken: "abc", wantErr: domain.ErrInvalidPageToken},
		{
			name: "all statuses with default page", caller: admin,
			setup: func(repo *mocks.MockReportRepository) {
				repo.EXPECT().List(gomock.Any(), domain.StatusUnspecified, domain.Page{Limit: 20, Offset: 0}).Return(items, int64(2), nil)
			},
			want: app.ListResult{Reports: items, TotalCount: 2, NextPageToken: ""},
		},
		{
			name: "open filter with more pages", caller: admin, status: 1, pageSize: 2, pageToken: "2",
			setup: func(repo *mocks.MockReportRepository) {
				repo.EXPECT().List(gomock.Any(), domain.StatusOpen, domain.Page{Limit: 2, Offset: 2}).Return(items, int64(7), nil)
			},
			want: app.ListResult{Reports: items, TotalCount: 7, NextPageToken: "4"},
		},
		{
			name: "page size above the maximum is capped", caller: admin, status: 3, pageSize: 500,
			setup: func(repo *mocks.MockReportRepository) {
				repo.EXPECT().List(gomock.Any(), domain.StatusActioned, domain.Page{Limit: 100, Offset: 0}).Return(nil, int64(0), nil)
			},
			want: app.ListResult{TotalCount: 0},
		},
		{
			name: "repository error is returned", caller: admin, status: 2,
			setup: func(repo *mocks.MockReportRepository) {
				repo.EXPECT().List(gomock.Any(), domain.StatusDismissed, gomock.Any()).Return(nil, int64(0), errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newReports(t)
			if tt.setup != nil {
				tt.setup(repo)
			}

			got, err := svc.List(context.Background(), tt.caller, tt.status, tt.pageSize, tt.pageToken)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, app.ListResult{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReports_Dismiss(t *testing.T) {
	dismissed := storedReport(domain.StatusDismissed)
	dismissed.UpdatedAt = now.UTC()

	tests := []struct {
		name     string
		caller   domain.Caller
		reportID string
		setup    func(repo *mocks.MockReportRepository, clock *mocks.MockClock)
		wantErr  error
		want     domain.Report
	}{
		{name: "applicant is denied", caller: applicant, reportID: reportID, wantErr: domain.ErrAdminRoleRequired},
		{name: "employer is denied", caller: employer, reportID: reportID, wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the ID", caller: employer, reportID: "bad", wantErr: domain.ErrAdminRoleRequired},
		{name: "report ID not a UUID", caller: admin, reportID: "report-1", wantErr: domain.ErrInvalidReportID},
		{name: "empty report ID", caller: admin, reportID: "", wantErr: domain.ErrInvalidReportID},
		{
			name: "unknown report", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, _ *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(domain.Report{}, domain.ErrReportNotFound)
			},
			wantErr: domain.ErrReportNotFound,
		},
		{
			name: "open report is dismissed", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(storedReport(domain.StatusOpen), nil)
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Dismiss(gomock.Any(), dismissed).Return(dismissed, nil)
			},
			want: dismissed,
		},
		{
			name: "uppercase report ID is lowercased", caller: admin, reportID: strings.ToUpper(reportID),
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(storedReport(domain.StatusOpen), nil)
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Dismiss(gomock.Any(), dismissed).Return(dismissed, nil)
			},
			want: dismissed,
		},
		{
			name: "already dismissed report", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(storedReport(domain.StatusDismissed), nil)
				clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrReportNotOpen,
		},
		{
			name: "actioned report", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(storedReport(domain.StatusActioned), nil)
				clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrReportNotOpen,
		},
		{
			name: "another admin closed it first", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(storedReport(domain.StatusOpen), nil)
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().Dismiss(gomock.Any(), dismissed).Return(domain.Report{}, domain.ErrReportNotOpen)
			},
			wantErr: domain.ErrReportNotOpen,
		},
		{
			name: "lookup error is returned", caller: admin, reportID: reportID,
			setup: func(repo *mocks.MockReportRepository, _ *mocks.MockClock) {
				repo.EXPECT().GetByID(gomock.Any(), reportID).Return(domain.Report{}, errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, clock := newReports(t)
			if tt.setup != nil {
				tt.setup(repo, clock)
			}

			got, err := svc.Dismiss(context.Background(), tt.caller, tt.reportID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Report{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReports_RemoveJob(t *testing.T) {
	removal := domain.Removal{JobID: jobID, Reason: "fake company", RemovedBy: adminID, RemovedAt: now.UTC()}

	tests := []struct {
		name    string
		caller  domain.Caller
		jobID   string
		reason  string
		setup   func(repo *mocks.MockReportRepository, clock *mocks.MockClock)
		wantErr error
	}{
		{name: "applicant is denied", caller: applicant, jobID: jobID, reason: "fake company", wantErr: domain.ErrAdminRoleRequired},
		{name: "employer is denied", caller: employer, jobID: jobID, reason: "fake company", wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the input", caller: applicant, jobID: "bad", reason: "", wantErr: domain.ErrAdminRoleRequired},
		{name: "job ID not a UUID", caller: admin, jobID: "job-1", reason: "fake company", wantErr: domain.ErrInvalidJobID},
		{name: "empty reason", caller: admin, jobID: jobID, reason: "", wantErr: domain.ErrInvalidRemovalReason},
		{name: "reason one over the maximum", caller: admin, jobID: jobID, reason: strings.Repeat("a", 501), wantErr: domain.ErrInvalidRemovalReason},
		{
			name: "admin removes the job", caller: admin, jobID: jobID, reason: "fake company",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().RemoveJob(gomock.Any(), removal).Return(nil)
			},
		},
		{
			name: "job ID lowercased and reason trimmed", caller: admin, jobID: strings.ToUpper(jobID), reason: "  fake company ",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().RemoveJob(gomock.Any(), removal).Return(nil)
			},
		},
		{
			name: "reason at the maximum", caller: admin, jobID: jobID, reason: strings.Repeat("a", 500),
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().RemoveJob(gomock.Any(), domain.Removal{
					JobID: jobID, Reason: strings.Repeat("a", 500), RemovedBy: adminID, RemovedAt: now.UTC(),
				}).Return(nil)
			},
		},
		{
			name: "second takedown of the same job", caller: admin, jobID: jobID, reason: "fake company",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().RemoveJob(gomock.Any(), removal).Return(domain.ErrJobAlreadyRemoved)
			},
			wantErr: domain.ErrJobAlreadyRemoved,
		},
		{
			name: "repository error is returned", caller: admin, jobID: jobID, reason: "fake company",
			setup: func(repo *mocks.MockReportRepository, clock *mocks.MockClock) {
				clock.EXPECT().Now().Return(now)
				repo.EXPECT().RemoveJob(gomock.Any(), removal).Return(errDB)
			},
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, clock := newReports(t)
			if tt.setup != nil {
				tt.setup(repo, clock)
			}

			got, err := svc.RemoveJob(context.Background(), tt.caller, tt.jobID, tt.reason)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, jobID, got)
		})
	}
}
