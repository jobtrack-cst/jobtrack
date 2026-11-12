package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	interviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	adapter "github.com/jobtrack-cst/jobtrack/services/interview/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/ports/mocks"
)

const (
	appID       = "33333333-3333-4333-8333-333333333333"
	interviewID = "55555555-5555-4555-8555-555555555555"
	applicantID = "11111111-1111-4111-8111-111111111111"
	employerID  = "22222222-2222-4222-8222-222222222222"
	otherID     = "44444444-4444-4444-8444-444444444444"
)

var (
	errDB        = errors.New("pq: secret SQL detail")
	now          = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	participants = domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID}
)

type fixture struct {
	h            *adapter.InterviewHandler
	participants *mocks.MockParticipantRepository
	interviews   *mocks.MockInterviewRepository
}

// Handlers are tested with the real app layer and mocked repositories, with
// "now" fixed at 12 Nov 2026 12:00 UTC.
func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	f := fixture{participants: mocks.NewMockParticipantRepository(ctrl), interviews: mocks.NewMockInterviewRepository(ctrl)}
	f.h = adapter.NewInterviewHandler(app.NewInterviews(f.participants, f.interviews, clock))
	return f
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asOwner() context.Context     { return withCaller(employerID, "employer") }
func asApplicant() context.Context { return withCaller(applicantID, "applicant") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func scheduleReq(at *timestamppb.Timestamp) *interviewv1.ScheduleInterviewRequest {
	return &interviewv1.ScheduleInterviewRequest{ApplicationId: appID, ScheduledAt: at, Details: " Room 3 "}
}

func stored() domain.Interview {
	return domain.Interview{InterviewID: interviewID, ApplicationID: appID, EmployerID: employerID, ApplicantID: applicantID,
		ScheduledAt: now.Add(48 * time.Hour), Details: "Room 3", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
}

var calls = map[string]func(*adapter.InterviewHandler, context.Context) error{
	"ScheduleInterview": func(h *adapter.InterviewHandler, c context.Context) error {
		_, err := h.ScheduleInterview(c, scheduleReq(timestamppb.New(now.Add(time.Hour))))
		return err
	},
	"GetInterview": func(h *adapter.InterviewHandler, c context.Context) error {
		_, err := h.GetInterview(c, &interviewv1.GetInterviewRequest{InterviewId: interviewID})
		return err
	},
	"ListInterviews": func(h *adapter.InterviewHandler, c context.Context) error {
		_, err := h.ListInterviews(c, &interviewv1.ListInterviewsRequest{ApplicationId: appID})
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

func TestHandler_RoleChecks(t *testing.T) {
	for rpc, call := range calls {
		t.Run("admin/"+rpc, func(t *testing.T) {
			wantCode(t, call(newFixture(t).h, withCaller(otherID, "admin")), codes.PermissionDenied)
		})
	}
	t.Run("applicant cannot schedule, even with bad input", func(t *testing.T) {
		_, err := newFixture(t).h.ScheduleInterview(asApplicant(), &interviewv1.ScheduleInterviewRequest{})
		wantCode(t, err, codes.PermissionDenied)
	})
}

func TestHandler_ScheduleInterview_TimeRules(t *testing.T) {
	for _, tt := range []struct {
		name string
		at   *timestamppb.Timestamp
		want codes.Code
	}{
		{"missing", nil, codes.InvalidArgument},
		{"out of range", &timestamppb.Timestamp{Seconds: 1, Nanos: -1}, codes.InvalidArgument},
		{"in the past", timestamppb.New(now.Add(-time.Hour)), codes.InvalidArgument},
		{"exactly now", timestamppb.New(now), codes.InvalidArgument},
		{"one microsecond ahead", timestamppb.New(now.Add(time.Microsecond)), codes.OK},
		{"exactly 365 days ahead", timestamppb.New(now.Add(365 * 24 * time.Hour)), codes.OK},
		{"over 365 days ahead", timestamppb.New(now.Add(365*24*time.Hour + time.Microsecond)), codes.InvalidArgument},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.want == codes.OK {
				f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
				f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			}
			_, err := f.h.ScheduleInterview(asOwner(), scheduleReq(tt.at))
			wantCode(t, err, tt.want)
		})
	}
}

func TestHandler_ScheduleInterview_MapsRequestAndResponse(t *testing.T) {
	f := newFixture(t)
	at := now.Add(48 * time.Hour)
	f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
	f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, iv domain.Interview) error {
		assert.Equal(t, employerID, iv.EmployerID, "employer comes from metadata")
		assert.Equal(t, applicantID, iv.ApplicantID, "applicant comes from the projection")
		return nil
	})
	resp, err := f.h.ScheduleInterview(asOwner(), scheduleReq(timestamppb.New(at)))
	require.NoError(t, err)
	iv := resp.GetInterview()
	assert.NotEmpty(t, iv.GetInterviewId())
	assert.Equal(t, appID, iv.GetApplicationId())
	assert.Equal(t, employerID, iv.GetEmployerId())
	assert.Equal(t, applicantID, iv.GetApplicantId())
	assert.Equal(t, at, iv.GetScheduledAt().AsTime())
	assert.Equal(t, "Room 3", iv.GetDetails())
	assert.Equal(t, now, iv.GetTimestamps().GetCreatedAt().AsTime())
}

func TestHandler_ScheduleInterview_Errors(t *testing.T) {
	future := timestamppb.New(now.Add(time.Hour))
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *interviewv1.ScheduleInterviewRequest
		setup func(fixture)
		want  codes.Code
	}{
		{name: "details 1001", ctx: asOwner(), req: &interviewv1.ScheduleInterviewRequest{ApplicationId: appID, ScheduledAt: future, Details: strings.Repeat("a", 1001)}, want: codes.InvalidArgument},
		{name: "invalid application id", ctx: asOwner(), req: &interviewv1.ScheduleInterviewRequest{ApplicationId: "x", ScheduledAt: future}, want: codes.InvalidArgument},
		{name: "nil request", ctx: asOwner(), req: nil, want: codes.InvalidArgument},
		{name: "unknown or unprocessed application", ctx: asOwner(), req: scheduleReq(future), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		}, want: codes.NotFound},
		{name: "another employer's application", ctx: withCaller(otherID, "employer"), req: scheduleReq(future), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		}, want: codes.PermissionDenied},
		{name: "same application and time", ctx: asOwner(), req: scheduleReq(future), setup: func(f fixture) {
			f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
			f.interviews.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrInterviewExists)
		}, want: codes.AlreadyExists},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(f)
			}
			_, err := f.h.ScheduleInterview(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
		})
	}
}

func TestHandler_GetInterview(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{"owning employer", asOwner(), codes.OK},
		{"the applicant", asApplicant(), codes.OK},
		{"another employer", withCaller(otherID, "employer"), codes.PermissionDenied},
		{"another applicant", withCaller(otherID, "applicant"), codes.PermissionDenied},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.interviews.EXPECT().Get(gomock.Any(), interviewID).Return(stored(), nil)
			resp, err := f.h.GetInterview(tt.ctx, &interviewv1.GetInterviewRequest{InterviewId: interviewID})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				assert.Equal(t, now.Add(48*time.Hour), resp.GetInterview().GetScheduledAt().AsTime())
				assert.Equal(t, "Room 3", resp.GetInterview().GetDetails())
			}
		})
	}
	t.Run("unknown interview", func(t *testing.T) {
		f := newFixture(t)
		f.interviews.EXPECT().Get(gomock.Any(), interviewID).Return(domain.Interview{}, domain.ErrInterviewNotFound)
		_, err := f.h.GetInterview(asOwner(), &interviewv1.GetInterviewRequest{InterviewId: interviewID})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("invalid id", func(t *testing.T) {
		_, err := newFixture(t).h.GetInterview(asOwner(), &interviewv1.GetInterviewRequest{InterviewId: "abc"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestHandler_ListInterviews(t *testing.T) {
	t.Run("maps entries and pagination", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		f.interviews.EXPECT().ListByApplication(gomock.Any(), appID, domain.Page{Limit: 1}).Return([]domain.Interview{stored()}, 3, nil)
		resp, err := f.h.ListInterviews(asApplicant(), &interviewv1.ListInterviewsRequest{
			ApplicationId: appID, Pagination: &commonv1.PaginationRequest{PageSize: 1},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetInterviews(), 1)
		assert.Equal(t, interviewID, resp.GetInterviews()[0].GetInterviewId())
		assert.Equal(t, int64(3), resp.GetPagination().GetTotalCount())
		assert.Equal(t, "1", resp.GetPagination().GetNextPageToken())
	})
	t.Run("another employer is denied", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(participants, nil)
		_, err := f.h.ListInterviews(withCaller(otherID, "employer"), &interviewv1.ListInterviewsRequest{ApplicationId: appID})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("unknown application", func(t *testing.T) {
		f := newFixture(t)
		f.participants.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, domain.ErrApplicationNotFound)
		_, err := f.h.ListInterviews(asOwner(), &interviewv1.ListInterviewsRequest{ApplicationId: appID})
		wantCode(t, err, codes.NotFound)
	})
	for _, p := range []*commonv1.PaginationRequest{{PageSize: -1}, {PageToken: "abc"}} {
		t.Run("invalid pagination", func(t *testing.T) {
			_, err := newFixture(t).h.ListInterviews(asOwner(), &interviewv1.ListInterviewsRequest{ApplicationId: appID, Pagination: p})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
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
			f.interviews.EXPECT().Get(gomock.Any(), interviewID).Return(domain.Interview{}, tt.repoErr)
			_, err := f.h.GetInterview(asOwner(), &interviewv1.GetInterviewRequest{InterviewId: interviewID})
			wantCode(t, err, tt.wantCode)
			assert.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}
