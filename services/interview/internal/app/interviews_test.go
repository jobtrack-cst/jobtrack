package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/ports/mocks"
)

const (
	appID       = "aaaaaaaa-1111-4222-8333-444455556666"
	interviewID = "bbbbbbbb-1111-4222-8333-444455556666"
	applicantID = "3f2b8c1e-1111-4222-8333-444455556666"
	employerID  = "eeeeeeee-1111-4222-8333-444455556666"
	otherID     = "9a8b7c6d-1111-4222-8333-444455556666"
)

var (
	now            = time.Date(2026, 11, 12, 12, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	nowUTC         = now.UTC()
	owner          = domain.Caller{UserID: employerID, Role: domain.RoleEmployer}
	applicant      = domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}
	otherEmployer  = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	otherApplicant = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	admin          = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	participants   = domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
	errDB          = errors.New("database down")
)

type fixture struct {
	participants *mocks.MockParticipantRepository
	interviews   *mocks.MockInterviewRepository
	svc          *app.Interviews
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	f := fixture{participants: mocks.NewMockParticipantRepository(ctrl), interviews: mocks.NewMockInterviewRepository(ctrl)}
	f.svc = app.NewInterviews(f.participants, f.interviews, clock)
	return f
}

func stored() domain.Interview {
	return domain.Interview{InterviewID: interviewID, ApplicationID: appID, EmployerID: employerID, ApplicantID: applicantID,
		ScheduledAt: nowUTC.Add(48 * time.Hour), Details: "Room 3", CreatedAt: nowUTC, UpdatedAt: nowUTC}
}

func scheduleIn(at time.Time) domain.ScheduleInput {
	return domain.ScheduleInput{ApplicationID: appID, ScheduledAt: at, Details: " Room 3 "}
}

// Expectations not set up in a case fail the test if called, so the error
// cases also prove that nothing is stored or read.
func TestInterviews_Schedule(t *testing.T) {
	t.Run("owning employer schedules with participants from the projection", func(t *testing.T) {
		f := newFixture(t)
		at := nowUTC.Add(48 * time.Hour)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, iv domain.Interview) error {
			_, err := uuid.Parse(iv.InterviewID)
			assert.NoError(t, err)
			assert.Equal(t, employerID, iv.EmployerID, "employer comes from the caller")
			assert.Equal(t, applicantID, iv.ApplicantID, "applicant comes from the projection")
			assert.Equal(t, at, iv.ScheduledAt)
			assert.Equal(t, "Room 3", iv.Details)
			assert.Equal(t, nowUTC, iv.CreatedAt)
			assert.Equal(t, iv.CreatedAt, iv.UpdatedAt)
			return nil
		})
		got, err := f.svc.Schedule(context.Background(), owner, scheduleIn(at))
		require.NoError(t, err)
		assert.Equal(t, appID, got.ApplicationID)
	})

	tests := []struct {
		name    string
		caller  domain.Caller
		in      domain.ScheduleInput
		setup   func(fixture)
		wantErr error
	}{
		{name: "applicant denied", caller: applicant, in: scheduleIn(nowUTC.Add(time.Hour)), wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied", caller: admin, in: scheduleIn(nowUTC.Add(time.Hour)), wantErr: domain.ErrEmployerRoleRequired},
		{name: "role checked before input", caller: applicant, in: domain.ScheduleInput{}, wantErr: domain.ErrEmployerRoleRequired},
		{name: "time in the past", caller: owner, in: scheduleIn(nowUTC.Add(-time.Minute)), wantErr: domain.ErrScheduledAtNotFuture},
		{name: "time exactly now", caller: owner, in: scheduleIn(now), wantErr: domain.ErrScheduledAtNotFuture},
		{name: "time over 365 days ahead", caller: owner, in: scheduleIn(nowUTC.AddDate(1, 0, 1)), wantErr: domain.ErrScheduledAtTooFar},
		{name: "invalid application id", caller: owner, in: domain.ScheduleInput{ApplicationID: "x", ScheduledAt: nowUTC.Add(time.Hour)}, wantErr: domain.ErrInvalidApplicationID},
		{name: "unknown or unprocessed application", caller: owner, in: scheduleIn(nowUTC.Add(time.Hour)), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, wantErr: domain.ErrApplicationNotFound},
		{name: "another employer's application", caller: otherEmployer, in: scheduleIn(nowUTC.Add(time.Hour)), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, wantErr: domain.ErrNotOwningEmployer},
		{name: "same application and time", caller: owner, in: scheduleIn(nowUTC.Add(time.Hour)), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrInterviewExists)
		}, wantErr: domain.ErrInterviewExists},
		{name: "repository error", caller: owner, in: scheduleIn(nowUTC.Add(time.Hour)), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errDB)
		}, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.svc.Schedule(context.Background(), tt.caller, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, domain.Interview{}, got)
		})
	}
}

func TestInterviews_Get(t *testing.T) {
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		lookup  bool
		repoErr error
		wantErr error
	}{
		{"owning employer reads", owner, interviewID, true, nil, nil},
		{"applicant reads", applicant, "BBBBBBBB-1111-4222-8333-444455556666", true, nil, nil},
		{"another employer denied", otherEmployer, interviewID, true, nil, domain.ErrNotParticipant},
		{"another applicant denied", otherApplicant, interviewID, true, nil, domain.ErrNotParticipant},
		{"admin denied before lookup", admin, interviewID, false, nil, domain.ErrRoleNotAllowed},
		{"invalid id", owner, "abc", false, nil, domain.ErrInvalidInterviewID},
		{"unknown interview", owner, interviewID, true, domain.ErrInterviewNotFound, domain.ErrInterviewNotFound},
		{"repository error", owner, interviewID, true, errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.lookup {
				result := stored()
				if tt.repoErr != nil {
					result = domain.Interview{}
				}
				f.interviews.EXPECT().Get(gomock.Any(), interviewID).Return(result, tt.repoErr)
			}
			got, err := f.svc.Get(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Interview{}, got, "nothing is returned on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stored(), got)
		})
	}
}

func TestInterviews_List(t *testing.T) {
	t.Run("applicant and owning employer list with paging", func(t *testing.T) {
		for _, c := range []domain.Caller{applicant, owner} {
			f := newFixture(t)
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.interviews.EXPECT().ListByApplication(gomock.Any(), appID, domain.Page{Limit: 1}).Return([]domain.Interview{stored()}, 2, nil)
			got, err := f.svc.List(context.Background(), c, appID, 1, "")
			require.NoError(t, err)
			assert.Equal(t, 2, got.TotalCount)
			assert.Equal(t, "1", got.NextPageToken)
			assert.Len(t, got.Interviews, 1)
		}
	})
	t.Run("default page size and last page", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.interviews.EXPECT().ListByApplication(gomock.Any(), appID, domain.Page{Limit: 20}).Return([]domain.Interview{}, 0, nil)
		got, err := f.svc.List(context.Background(), owner, appID, 0, "")
		require.NoError(t, err)
		assert.Equal(t, "", got.NextPageToken)
	})
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		size    int32
		token   string
		setup   func(fixture)
		wantErr error
	}{
		{name: "admin denied before anything", caller: admin, id: appID, wantErr: domain.ErrRoleNotAllowed},
		{name: "invalid application id", caller: owner, id: "", wantErr: domain.ErrInvalidApplicationID},
		{name: "negative page size", caller: owner, id: appID, size: -1, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: owner, id: appID, token: "abc", wantErr: domain.ErrInvalidPageToken},
		{name: "unknown application", caller: owner, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, wantErr: domain.ErrApplicationNotFound},
		{name: "another employer denied, nothing listed", caller: otherEmployer, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, wantErr: domain.ErrNotParticipant},
		{name: "another applicant denied", caller: otherApplicant, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, wantErr: domain.ErrNotParticipant},
		{name: "repository error", caller: applicant, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.interviews.EXPECT().ListByApplication(gomock.Any(), appID, gomock.Any()).Return(nil, 0, errDB)
		}, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			_, err := f.svc.List(context.Background(), tt.caller, tt.id, tt.size, tt.token)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
