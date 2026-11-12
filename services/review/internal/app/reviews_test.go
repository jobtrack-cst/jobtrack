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

	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/ports/mocks"
)

const (
	appID       = "aaaaaaaa-1111-4222-8333-444455556666"
	applicantID = "3f2b8c1e-1111-4222-8333-444455556666"
	employerID  = "eeeeeeee-1111-4222-8333-444455556666"
	otherID     = "9a8b7c6d-1111-4222-8333-444455556666"
)

var (
	now           = time.Date(2026, 11, 12, 12, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	owner         = domain.Caller{UserID: employerID, Role: domain.RoleEmployer}
	otherEmployer = domain.Caller{UserID: otherID, Role: domain.RoleEmployer}
	applicant     = domain.Caller{UserID: applicantID, Role: domain.RoleApplicant}
	admin         = domain.Caller{UserID: otherID, Role: domain.RoleAdmin}
	participants  = domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
	errDB         = errors.New("database down")
)

type fixture struct {
	participants *mocks.MockParticipantRepository
	reviews      *mocks.MockReviewRepository
	svc          *app.Reviews
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	f := fixture{participants: mocks.NewMockParticipantRepository(ctrl), reviews: mocks.NewMockReviewRepository(ctrl)}
	f.svc = app.NewReviews(f.participants, f.reviews, clock)
	return f
}

func validInput() domain.ReviewInput {
	return domain.ReviewInput{ApplicationID: appID, Score: 4, Decision: domain.DecisionShortlisted}
}

// Expectations not set up in a case fail the test if called, so the error
// cases also prove that nothing is stored or read.
func TestReviews_Complete(t *testing.T) {
	t.Run("owning employer completes the review", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r domain.Review) error {
			_, err := uuid.Parse(r.ReviewID)
			assert.NoError(t, err, "review_id is a generated UUID")
			assert.Equal(t, appID, r.ApplicationID)
			assert.Equal(t, employerID, r.EmployerID, "employer comes from the caller")
			assert.Equal(t, int32(4), r.Score)
			assert.Equal(t, domain.DecisionShortlisted, r.Decision)
			assert.Equal(t, now.UTC(), r.CreatedAt)
			assert.Equal(t, time.UTC, r.CreatedAt.Location())
			assert.Equal(t, r.CreatedAt, r.UpdatedAt)
			return nil
		})
		got, err := f.svc.Complete(context.Background(), owner, validInput())
		require.NoError(t, err)
		assert.Equal(t, employerID, got.EmployerID)
	})

	tests := []struct {
		name    string
		caller  domain.Caller
		in      domain.ReviewInput
		setup   func(fixture)
		wantErr error
	}{
		{name: "applicant denied before anything", caller: applicant, in: validInput(), wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied before anything", caller: admin, in: validInput(), wantErr: domain.ErrEmployerRoleRequired},
		{name: "role checked before input", caller: applicant, in: domain.ReviewInput{Score: 0}, wantErr: domain.ErrEmployerRoleRequired},
		{name: "score 0", caller: owner, in: domain.ReviewInput{ApplicationID: appID, Score: 0, Decision: domain.DecisionRejected}, wantErr: domain.ErrInvalidScore},
		{name: "score 6", caller: owner, in: domain.ReviewInput{ApplicationID: appID, Score: 6, Decision: domain.DecisionRejected}, wantErr: domain.ErrInvalidScore},
		{name: "decision unspecified", caller: owner, in: domain.ReviewInput{ApplicationID: appID, Score: 3}, wantErr: domain.ErrInvalidDecision},
		{name: "invalid application id", caller: owner, in: domain.ReviewInput{ApplicationID: "x", Score: 3, Decision: domain.DecisionRejected}, wantErr: domain.ErrInvalidApplicationID},
		{name: "unknown or unprocessed application", caller: owner, in: validInput(), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, wantErr: domain.ErrApplicationNotFound},
		{name: "another employer's application", caller: otherEmployer, in: validInput(), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, wantErr: domain.ErrNotOwningEmployer},
		{name: "second review", caller: owner, in: validInput(), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrReviewExists)
		}, wantErr: domain.ErrReviewExists},
		{name: "participants repository error", caller: owner, in: validInput(), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, errDB)
		}, wantErr: errDB},
		{name: "reviews repository error", caller: owner, in: validInput(), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errDB)
		}, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.svc.Complete(context.Background(), tt.caller, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, domain.Review{}, got)
		})
	}
}

func TestReviews_Get(t *testing.T) {
	stored := domain.Review{ReviewID: uuid.NewString(), ApplicationID: appID, EmployerID: employerID, Score: 2, Decision: domain.DecisionRejected, CreatedAt: now, UpdatedAt: now}
	tests := []struct {
		name    string
		caller  domain.Caller
		id      string
		setup   func(fixture)
		wantErr error
	}{
		{name: "owning employer reads the review", caller: owner, id: "AAAAAAAA-1111-4222-8333-444455556666", setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(stored, nil)
		}},
		{name: "applicant denied before lookup", caller: applicant, id: appID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "admin denied before lookup", caller: admin, id: appID, wantErr: domain.ErrEmployerRoleRequired},
		{name: "invalid id", caller: owner, id: "abc", wantErr: domain.ErrInvalidApplicationID},
		{name: "unknown application", caller: owner, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, wantErr: domain.ErrApplicationNotFound},
		{name: "another employer denied, review not read", caller: otherEmployer, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, wantErr: domain.ErrNotOwningEmployer},
		{name: "no review yet", caller: owner, id: appID, setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.Review{}, domain.ErrReviewNotFound)
		}, wantErr: domain.ErrReviewNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			got, err := f.svc.Get(context.Background(), tt.caller, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.Review{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stored, got)
		})
	}
}
