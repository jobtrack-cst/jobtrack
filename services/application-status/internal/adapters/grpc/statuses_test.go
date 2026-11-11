package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	asv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/application-status/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/ports/mocks"
)

const (
	appID       = "33333333-3333-4333-8333-333333333333"
	applicantID = "11111111-1111-4111-8111-111111111111"
	employerID  = "22222222-2222-4222-8222-222222222222"
	otherID     = "44444444-4444-4444-8444-444444444444"
)

var (
	errDB = errors.New("pq: secret SQL detail")
	now   = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
)

// Handlers are tested with the real app layer and a mocked repository. Any
// repository call a case does not expect fails the test.
func newHandler(t *testing.T) (*adapter.ApplicationStatusHandler, *mocks.MockStatusRepository) {
	repo := mocks.NewMockStatusRepository(gomock.NewController(t))
	return adapter.NewApplicationStatusHandler(app.NewStatuses(repo)), repo
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func stored(s domain.Status) domain.ApplicationStatus {
	return domain.ApplicationStatus{
		ApplicationID: appID, ApplicantID: applicantID, EmployerID: employerID,
		Status: s, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
}

var calls = map[string]func(*adapter.ApplicationStatusHandler, context.Context) error{
	"GetApplicationStatus": func(h *adapter.ApplicationStatusHandler, c context.Context) error {
		_, err := h.GetApplicationStatus(c, &asv1.GetApplicationStatusRequest{ApplicationId: appID})
		return err
	},
	"ListApplicationStatusHistory": func(h *adapter.ApplicationStatusHandler, c context.Context) error {
		_, err := h.ListApplicationStatusHistory(c, &asv1.ListApplicationStatusHistoryRequest{ApplicationId: appID})
		return err
	},
}

func TestHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", applicantID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "applicant"))},
		{"user ID not a UUID", withCaller("abc", "applicant")},
		{"unknown role", withCaller(applicantID, "superuser")},
		{"uppercase role", withCaller(applicantID, "Applicant")},
	} {
		for rpc, call := range calls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, _ := newHandler(t)
				wantCode(t, call(h, tt.ctx), codes.Unauthenticated)
			})
		}
	}
}

// Admins are denied before any lookup, even with an invalid ID.
func TestHandler_AdminDeniedBeforeInput(t *testing.T) {
	for rpc, call := range calls {
		t.Run(rpc, func(t *testing.T) {
			h, _ := newHandler(t)
			wantCode(t, call(h, withCaller(applicantID, "admin")), codes.PermissionDenied)
		})
	}
	h, _ := newHandler(t)
	_, err := h.GetApplicationStatus(withCaller(applicantID, "admin"), &asv1.GetApplicationStatusRequest{ApplicationId: "bad"})
	wantCode(t, err, codes.PermissionDenied)
}

func TestHandler_ReaderMatrix(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{"the applicant", withCaller(applicantID, "applicant"), codes.OK},
		{"the employer", withCaller(employerID, "employer"), codes.OK},
		{"another applicant", withCaller(otherID, "applicant"), codes.PermissionDenied},
		{"another employer", withCaller(otherID, "employer"), codes.PermissionDenied},
		{"applicant ID as employer", withCaller(applicantID, "employer"), codes.PermissionDenied},
	} {
		for rpc, call := range calls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, repo := newHandler(t)
				repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusSubmitted), nil)
				if tt.want == codes.OK && rpc == "ListApplicationStatusHistory" {
					repo.EXPECT().ListHistory(gomock.Any(), appID, gomock.Any()).Return([]domain.HistoryEntry{}, 0, nil)
				}
				wantCode(t, call(h, tt.ctx), tt.want)
			})
		}
	}
}

func TestHandler_GetApplicationStatus_MapsEveryStatus(t *testing.T) {
	want := map[domain.Status]asv1.ApplicationStatus{
		domain.StatusSubmitted:   asv1.ApplicationStatus_APPLICATION_STATUS_SUBMITTED,
		domain.StatusUnderReview: asv1.ApplicationStatus_APPLICATION_STATUS_UNDER_REVIEW,
		domain.StatusShortlisted: asv1.ApplicationStatus_APPLICATION_STATUS_SHORTLISTED,
		domain.StatusInterview:   asv1.ApplicationStatus_APPLICATION_STATUS_INTERVIEW,
		domain.StatusOffered:     asv1.ApplicationStatus_APPLICATION_STATUS_OFFERED,
		domain.StatusAccepted:    asv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED,
		domain.StatusDeclined:    asv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
		domain.StatusRejected:    asv1.ApplicationStatus_APPLICATION_STATUS_REJECTED,
		domain.StatusWithdrawn:   asv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN,
	}
	require.Len(t, want, len(domain.AllStatuses))
	for s, protoStatus := range want {
		t.Run(s.String(), func(t *testing.T) {
			h, repo := newHandler(t)
			repo.EXPECT().Get(gomock.Any(), appID).Return(stored(s), nil)
			resp, err := h.GetApplicationStatus(withCaller(applicantID, "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: appID})
			require.NoError(t, err)
			r := resp.GetStatusRecord()
			assert.Equal(t, appID, r.GetApplicationId())
			assert.Equal(t, protoStatus, r.GetStatus())
			assert.Equal(t, now.Add(-time.Hour), r.GetTimestamps().GetCreatedAt().AsTime())
			assert.Equal(t, now, r.GetTimestamps().GetUpdatedAt().AsTime())
		})
	}
}

func TestHandler_GetApplicationStatus_Errors(t *testing.T) {
	t.Run("unknown application", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, domain.ErrApplicationNotFound)
		_, err := h.GetApplicationStatus(withCaller(applicantID, "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: appID})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("invalid id", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.GetApplicationStatus(withCaller(applicantID, "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: "abc"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("nil request", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.GetApplicationStatus(withCaller(applicantID, "applicant"), nil)
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestHandler_ListApplicationStatusHistory(t *testing.T) {
	t.Run("maps entries and pagination; first entry has UNSPECIFIED previous", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), appID).Return(stored(domain.StatusUnderReview), nil)
		repo.EXPECT().ListHistory(gomock.Any(), appID, domain.Page{Limit: 2, Offset: 0}).Return([]domain.HistoryEntry{
			{ApplicationID: appID, PreviousStatus: domain.StatusUnspecified, NewStatus: domain.StatusSubmitted, OccurredAt: now},
			{ApplicationID: appID, PreviousStatus: domain.StatusSubmitted, NewStatus: domain.StatusUnderReview, OccurredAt: now.Add(time.Minute)},
		}, 3, nil)
		resp, err := h.ListApplicationStatusHistory(withCaller(employerID, "employer"), &asv1.ListApplicationStatusHistoryRequest{
			ApplicationId: appID, Pagination: &commonv1.PaginationRequest{PageSize: 2},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetEntries(), 2)
		first, second := resp.GetEntries()[0], resp.GetEntries()[1]
		assert.Equal(t, asv1.ApplicationStatus_APPLICATION_STATUS_UNSPECIFIED, first.GetPreviousStatus())
		assert.Equal(t, asv1.ApplicationStatus_APPLICATION_STATUS_SUBMITTED, first.GetNewStatus())
		assert.Equal(t, now, first.GetOccurredAt().AsTime())
		assert.Equal(t, asv1.ApplicationStatus_APPLICATION_STATUS_SUBMITTED, second.GetPreviousStatus())
		assert.Equal(t, asv1.ApplicationStatus_APPLICATION_STATUS_UNDER_REVIEW, second.GetNewStatus())
		assert.Equal(t, int64(3), resp.GetPagination().GetTotalCount())
		assert.Equal(t, "2", resp.GetPagination().GetNextPageToken())
	})
	for _, p := range []*commonv1.PaginationRequest{{PageSize: -1}, {PageToken: "abc"}} {
		t.Run("invalid pagination", func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.ListApplicationStatusHistory(withCaller(applicantID, "applicant"), &asv1.ListApplicationStatusHistoryRequest{ApplicationId: appID, Pagination: p})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
	t.Run("unknown application", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, domain.ErrApplicationNotFound)
		_, err := h.ListApplicationStatusHistory(withCaller(applicantID, "applicant"), &asv1.ListApplicationStatusHistoryRequest{ApplicationId: appID})
		wantCode(t, err, codes.NotFound)
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
			h, repo := newHandler(t)
			repo.EXPECT().Get(gomock.Any(), appID).Return(domain.ApplicationStatus{}, tt.repoErr)
			_, err := h.GetApplicationStatus(withCaller(applicantID, "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: appID})
			wantCode(t, err, tt.wantCode)
			assert.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}

// AGENTS.md Section 6: any other transition must be rejected with
// FAILED_PRECONDITION. Transitions have no RPC yet, so the mapping is checked
// directly for the errors the Sprint 2 consumers will return.
func TestToStatus_TransitionCodes(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want codes.Code
	}{
		{domain.ErrInvalidTransition, codes.FailedPrecondition},
		{domain.ErrStatusChanged, codes.FailedPrecondition},
		{domain.ErrApplicationExists, codes.AlreadyExists},
		{domain.ErrInvalidParticipantID, codes.InvalidArgument},
		{domain.ErrInvalidTime, codes.InvalidArgument},
	} {
		t.Run(tt.err.Error(), func(t *testing.T) {
			wantCode(t, adapter.ToStatus(tt.err), tt.want)
		})
	}
}
