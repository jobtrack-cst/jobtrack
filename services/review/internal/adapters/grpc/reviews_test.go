package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	reviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/review/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/ports/mocks"
)

const (
	appID       = "33333333-3333-4333-8333-333333333333"
	applicantID = "11111111-1111-4111-8111-111111111111"
	employerID  = "22222222-2222-4222-8222-222222222222"
	otherID     = "44444444-4444-4444-8444-444444444444"
	reviewID    = "55555555-5555-4555-8555-555555555555"
)

var (
	errDB        = errors.New("pq: secret SQL detail")
	now          = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	participants = domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
)

type fixture struct {
	h            *adapter.ReviewHandler
	participants *mocks.MockParticipantRepository
	reviews      *mocks.MockReviewRepository
}

// Handlers are tested with the real app layer and mocked repositories. Any
// repository call a case does not expect fails the test.
func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	f := fixture{participants: mocks.NewMockParticipantRepository(ctrl), reviews: mocks.NewMockReviewRepository(ctrl)}
	f.h = adapter.NewReviewHandler(app.NewReviews(f.participants, f.reviews, clock))
	return f
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asOwner() context.Context { return withCaller(employerID, "employer") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func completeReq(score int32, d reviewv1.ReviewDecision) *reviewv1.CompleteReviewRequest {
	return &reviewv1.CompleteReviewRequest{ApplicationId: appID, Score: score, Decision: d}
}

var calls = map[string]func(*adapter.ReviewHandler, context.Context) error{
	"CompleteReview": func(h *adapter.ReviewHandler, c context.Context) error {
		_, err := h.CompleteReview(c, completeReq(3, reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED))
		return err
	},
	"GetReview": func(h *adapter.ReviewHandler, c context.Context) error {
		_, err := h.GetReview(c, &reviewv1.GetReviewRequest{ApplicationId: appID})
		return err
	},
}

func TestHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", employerID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "employer"))},
		{"user ID not a UUID", withCaller("abc", "employer")},
		{"unknown role", withCaller(employerID, "superuser")},
		{"uppercase role", withCaller(employerID, "Employer")},
	} {
		for rpc, call := range calls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				wantCode(t, call(newFixture(t).h, tt.ctx), codes.Unauthenticated)
			})
		}
	}
}

// Applicants and admins are denied before any input or lookup.
func TestHandler_OtherRolesDenied(t *testing.T) {
	for _, role := range []string{"applicant", "admin"} {
		for rpc, call := range calls {
			t.Run(role+"/"+rpc, func(t *testing.T) {
				wantCode(t, call(newFixture(t).h, withCaller(applicantID, role)), codes.PermissionDenied)
			})
		}
	}
	_, err := newFixture(t).h.CompleteReview(withCaller(applicantID, "applicant"), completeReq(0, 0))
	wantCode(t, err, codes.PermissionDenied)
}

func TestHandler_CompleteReview_ScoreBoundaries(t *testing.T) {
	for _, tt := range []struct {
		score int32
		want  codes.Code
	}{{0, codes.InvalidArgument}, {1, codes.OK}, {5, codes.OK}, {6, codes.InvalidArgument}} {
		t.Run("score", func(t *testing.T) {
			f := newFixture(t)
			if tt.want == codes.OK {
				f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
				f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			}
			resp, err := f.h.CompleteReview(asOwner(), completeReq(tt.score, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED))
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				assert.Equal(t, tt.score, resp.GetReview().GetScore())
			}
		})
	}
}

func TestHandler_CompleteReview_MapsRequestAndResponse(t *testing.T) {
	for _, d := range []reviewv1.ReviewDecision{reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED} {
		t.Run(d.String(), func(t *testing.T) {
			f := newFixture(t)
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r domain.Review) error {
				assert.Equal(t, employerID, r.EmployerID, "employer comes from metadata")
				return nil
			})
			resp, err := f.h.CompleteReview(asOwner(), completeReq(4, d))
			require.NoError(t, err)
			r := resp.GetReview()
			assert.NotEmpty(t, r.GetReviewId())
			assert.Equal(t, appID, r.GetApplicationId())
			assert.Equal(t, employerID, r.GetEmployerId())
			assert.Equal(t, int32(4), r.GetScore())
			assert.Equal(t, d, r.GetDecision())
			assert.Equal(t, now, r.GetTimestamps().GetCreatedAt().AsTime())
			assert.Equal(t, now, r.GetTimestamps().GetUpdatedAt().AsTime())
		})
	}
}

func TestHandler_CompleteReview_Errors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *reviewv1.CompleteReviewRequest
		setup func(fixture)
		want  codes.Code
	}{
		{name: "decision unspecified", ctx: asOwner(), req: completeReq(3, reviewv1.ReviewDecision_REVIEW_DECISION_UNSPECIFIED), want: codes.InvalidArgument},
		{name: "unknown decision value", ctx: asOwner(), req: completeReq(3, reviewv1.ReviewDecision(99)), want: codes.InvalidArgument},
		{name: "invalid application id", ctx: asOwner(), req: &reviewv1.CompleteReviewRequest{ApplicationId: "x", Score: 3, Decision: reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED}, want: codes.InvalidArgument},
		{name: "nil request", ctx: asOwner(), req: nil, want: codes.InvalidArgument},
		{name: "unknown or unprocessed application", ctx: asOwner(), req: completeReq(3, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, want: codes.NotFound},
		{name: "another employer's application", ctx: withCaller(otherID, "employer"), req: completeReq(3, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, want: codes.PermissionDenied},
		{name: "second review", ctx: asOwner(), req: completeReq(3, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.reviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrReviewExists)
		}, want: codes.AlreadyExists},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			_, err := f.h.CompleteReview(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
		})
	}
}

func TestHandler_GetReview(t *testing.T) {
	stored := domain.Review{ReviewID: reviewID, ApplicationID: appID, EmployerID: employerID, Score: 2,
		Decision: domain.DecisionRejected, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	t.Run("owning employer reads the review", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.reviews.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(stored, nil)
		resp, err := f.h.GetReview(asOwner(), &reviewv1.GetReviewRequest{ApplicationId: appID})
		require.NoError(t, err)
		assert.Equal(t, reviewID, resp.GetReview().GetReviewId())
		assert.Equal(t, reviewv1.ReviewDecision_REVIEW_DECISION_REJECTED, resp.GetReview().GetDecision())
		assert.Equal(t, now.Add(-time.Hour), resp.GetReview().GetTimestamps().GetCreatedAt().AsTime())
	})
	t.Run("another employer is denied", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		_, err := f.h.GetReview(withCaller(otherID, "employer"), &reviewv1.GetReviewRequest{ApplicationId: appID})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("no review yet", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.reviews.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.Review{}, domain.ErrReviewNotFound)
		_, err := f.h.GetReview(asOwner(), &reviewv1.GetReviewRequest{ApplicationId: appID})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("invalid id", func(t *testing.T) {
		_, err := newFixture(t).h.GetReview(asOwner(), &reviewv1.GetReviewRequest{ApplicationId: "abc"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestHandler_ErrorsDoNotLeak(t *testing.T) {
	for _, tt := range []struct {
		name     string
		repoErr  error
		wantCode codes.Code
		wantMsg  string
	}{
		{"database error", errDB, codes.Internal, "internal error"},
		{"canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, tt.repoErr)
			_, err := f.h.GetReview(asOwner(), &reviewv1.GetReviewRequest{ApplicationId: appID})
			wantCode(t, err, tt.wantCode)
			assert.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}
