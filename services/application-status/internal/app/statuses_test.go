package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports/mocks"
)

const (
	appID       = "aaaaaaaa-1111-4222-8333-444455556666"
	applicantID = "3f2b8c1e-1111-4222-8333-444455556666"
	employerID  = "eeeeeeee-1111-4222-8333-444455556666"
	otherID     = "9a8b7c6d-1111-4222-8333-444455556666"
)

var (
	at             = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	applicant      = domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}
	employer       = domain.Caller{UserID: employerID, Role: domain.RoleEmployer}
	otherApplicant = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	otherEmployer  = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	admin          = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	errDB          = errors.New("database down")
)

func newFixture(t *testing.T) (*mocks.MockStatusRepository, *app.Statuses) {
	repo := mocks.NewMockStatusRepository(gomock.NewController(t))
	return repo, app.NewStatuses(repo)
}

func stored(status domain.Status) domain.ApplicationStatus {
	return domain.ApplicationStatus{
		ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID,
		Status: status, CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Hour),
	}
}

// Expectations not set up in a case fail the test if called, so the error
// cases also prove that nothing is stored or read.
func TestStatuses_Start(t *testing.T) {
	t.Run("records SUBMITTED with the first history entry", func(t *testing.T) {
		repo, svc := newFixture(t)
		want := domain.ApplicationStatus{
			ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID,
			Status: domain.StatusSubmitted, CreatedAt: at, UpdatedAt: at,
		}
		repo.EXPECT().Create(gomock.Any(), want, domain.HistoryEntry{
			ApplicationID: appID, PreviousStatus: domain.StatusUnspecified, NewStatus: domain.StatusSubmitted, OccurredAt: at,
		}).Return(nil)
		got, err := svc.Start(context.Background(), appID, applicantID, employerID, at)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("already recorded", func(t *testing.T) {
		repo, svc := newFixture(t)
		repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.ErrApplicationExists)
		_, err := svc.Start(context.Background(), appID, applicantID, employerID, at)
		require.ErrorIs(t, err, domain.ErrApplicationExists)
	})
	t.Run("invalid input stores nothing", func(t *testing.T) {
		_, svc := newFixture(t)
		_, err := svc.Start(context.Background(), appID, "bad", employerID, at)
		require.ErrorIs(t, err, domain.ErrInvalidParticipantID)
	})
}

func TestStatuses_Transition(t *testing.T) {
	t.Run("allowed transition is stored with its history entry", func(t *testing.T) {
		repo, svc := newFixture(t)
		later := at.Add(time.Minute)
		repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusInterview), nil)
		want := stored(domain.StatusOffered)
		want.UpdatedAt = later
		repo.EXPECT().Transition(gomock.Any(), want, domain.StatusInterview, domain.HistoryEntry{
			ApplicationID: appID, PreviousStatus: domain.StatusInterview, NewStatus: domain.StatusOffered, OccurredAt: later,
		}).Return(nil)
		got, err := svc.Transition(context.Background(), "AAAAAAAA-1111-4222-8333-444455556666", domain.StatusOffered, later)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("disallowed transition is FAILED_PRECONDITION and stores nothing", func(t *testing.T) {
		repo, svc := newFixture(t)
		repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusAccepted), nil)
		_, err := svc.Transition(context.Background(), appID, domain.StatusWithdrawn, at)
		require.ErrorIs(t, err, domain.ErrInvalidTransition)
	})
	tests := []struct {
		name    string
		id      string
		setup   func(*mocks.MockStatusRepository)
		wantErr error
	}{
		{"invalid id", "x", nil, domain.ErrInvalidApplicationID},
		{"unknown application", appID, func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, domain.ErrApplicationNotFound)
		}, domain.ErrApplicationNotFound},
		{"concurrent change", appID, func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusSubmitted), nil)
			r.EXPECT().Transition(gomock.Any(), gomock.Any(), domain.StatusSubmitted, gomock.Any()).Return(domain.ErrStatusChanged)
		}, domain.ErrStatusChanged},
		{"repository error", appID, func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.setup != nil {
				tt.setup(repo)
			}
			_, err := svc.Transition(context.Background(), tt.id, domain.StatusUnderReview, at)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestStatuses_GetStatus(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		lookup  bool
		repoErr error
		wantErr error
	}{
		{"applicant reads", applicant, appID, true, nil, nil},
		{"employer reads", employer, appID, true, nil, nil},
		{"uppercase id is normalized", applicant, "AAAAAAAA-1111-4222-8333-444455556666", true, nil, nil},
		{"other applicant denied", otherApplicant, appID, true, nil, domain.ErrNotParticipant},
		{"other employer denied", otherEmployer, appID, true, nil, domain.ErrNotParticipant},
		{"admin denied before lookup", admin, appID, false, nil, domain.ErrRoleNotAllowed},
		{"invalid id", applicant, "abc", false, nil, domain.ErrInvalidApplicationID},
		{"unknown application", applicant, appID, true, domain.ErrApplicationNotFound, domain.ErrApplicationNotFound},
		{"repository error", applicant, appID, true, errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.lookup {
				result := stored(domain.StatusShortlisted)
				if tt.repoErr != nil {
					result = domain.ApplicationStatus{}
				}
				repo.EXPECT().Get(gomock.Any(), appID).Return(result, tt.repoErr)
			}
			got, err := svc.GetStatus(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.ApplicationStatus{}, got, "nothing is returned on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stored(domain.StatusShortlisted), got)
		})
	}
}

func TestStatuses_ListHistory(t *testing.T) {
	entries := []domain.HistoryEntry{
		{ApplicationID: appID, PreviousStatus: domain.StatusUnspecified, NewStatus: domain.StatusSubmitted, OccurredAt: at},
		{ApplicationID: appID, PreviousStatus: domain.StatusSubmitted, NewStatus: domain.StatusUnderReview, OccurredAt: at.Add(time.Minute)},
	}
	t.Run("applicant and employer read the history with paging", func(t *testing.T) {
		for _, c := range []domain.Caller{applicant, employer} {
			repo, svc := newFixture(t)
			repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusUnderReview), nil)
			repo.EXPECT().ListHistory(gomock.Any(), appID, domain.Page{Limit: 2, Offset: 0}).Return(entries, 3, nil)
			got, err := svc.ListHistory(context.Background(), c, appID, 2, "")
			require.NoError(t, err)
			assert.Equal(t, entries, got.Entries)
			assert.Equal(t, 3, got.TotalCount)
			assert.Equal(t, "2", got.NextPageToken)
		}
	})
	t.Run("default page size and last page", func(t *testing.T) {
		repo, svc := newFixture(t)
		repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusUnderReview), nil)
		repo.EXPECT().ListHistory(gomock.Any(), appID, domain.Page{Limit: 20}).Return(entries, 2, nil)
		got, err := svc.ListHistory(context.Background(), applicant, appID, 0, "")
		require.NoError(t, err)
		assert.Equal(t, "", got.NextPageToken)
	})
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		size    int32
		token   string
		setup   func(*mocks.MockStatusRepository)
		wantErr error
	}{
		{"admin denied before anything", admin, appID, 0, "", nil, domain.ErrRoleNotAllowed},
		{"role checked before page", admin, appID, -1, "", nil, domain.ErrRoleNotAllowed},
		{"negative page size", applicant, appID, -1, "", nil, domain.ErrInvalidPageSize},
		{"bad page token", applicant, appID, 0, "abc", nil, domain.ErrInvalidPageToken},
		{"invalid id", applicant, "", 0, "", nil, domain.ErrInvalidApplicationID},
		{"unknown application", applicant, appID, 0, "", func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, domain.ErrApplicationNotFound)
		}, domain.ErrApplicationNotFound},
		{"other applicant denied, history not read", otherApplicant, appID, 0, "", func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusSubmitted), nil)
		}, domain.ErrNotParticipant},
		{"repository error", employer, appID, 0, "", func(r *mocks.MockStatusRepository) {
			r.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusSubmitted), nil)
			r.EXPECT().ListHistory(gomock.Any(), appID, gomock.Any()).Return(nil, 0, errDB)
		}, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, svc := newFixture(t)
			if tt.setup != nil {
				tt.setup(repo)
			}
			_, err := svc.ListHistory(context.Background(), tt.caller, tt.id, tt.size, tt.token)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
