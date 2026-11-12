package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	cv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/company_verification/v1"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/company-verification/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports/mocks"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	adminID    = "22222222-2222-4222-8222-222222222222"
	requestID  = "33333333-3333-4333-8333-333333333333"
)

var (
	errDB    = errors.New("pq: secret SQL detail")
	testTime = time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
)

type deps struct {
	repo  *mocks.MockRequestRepository
	clock *mocks.MockClock
}

// Handlers are tested with the real app layer and mocked ports.
func newHandler(t *testing.T) (*adapter.VerificationHandler, deps) {
	ctrl := gomock.NewController(t)
	d := deps{repo: mocks.NewMockRequestRepository(ctrl), clock: mocks.NewMockClock(ctrl)}
	return adapter.NewVerificationHandler(app.NewRequests(d.repo, d.clock)), d
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asEmployer() context.Context { return withCaller(employerID, "employer") }
func asAdmin() context.Context    { return withCaller(adminID, "admin") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func stored(s domain.RequestStatus) domain.VerificationRequest {
	return domain.VerificationRequest{
		ID: requestID, EmployerID: employerID, RegistrationNumber: "BT-1", Notes: "note",
		Status: s, CreatedAt: testTime, UpdatedAt: testTime,
	}
}

func returnArg(_ context.Context, r domain.VerificationRequest) (domain.VerificationRequest, error) {
	return r, nil
}

func returnDecided(_ context.Context, r domain.VerificationRequest, _ *eventsv1.EventEnvelope) (domain.VerificationRequest, error) {
	return r, nil
}

// identityCases are the caller checks every RPC shares.
var identityCases = []struct {
	name string
	ctx  context.Context
}{
	{"no metadata", context.Background()},
	{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", employerID))},
	{"user ID not a UUID", withCaller("abc", "admin")},
	{"unknown role", withCaller(adminID, "superuser")},
}

func TestVerificationHandler_RejectsBadIdentityOnEveryRPC(t *testing.T) {
	for _, tt := range identityCases {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.SubmitVerificationRequest(tt.ctx, &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1"})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.ListMyVerificationRequests(tt.ctx, &cv1.ListMyVerificationRequestsRequest{})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.ListVerificationRequests(tt.ctx, &cv1.ListVerificationRequestsRequest{})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.ApproveVerificationRequest(tt.ctx, &cv1.ApproveVerificationRequestRequest{RequestId: requestID})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.RejectVerificationRequest(tt.ctx, &cv1.RejectVerificationRequestRequest{RequestId: requestID, Reason: "x"})
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

// Role matrix: which roles each RPC refuses with PERMISSION_DENIED.
func TestVerificationHandler_RoleChecks(t *testing.T) {
	calls := map[string]func(*adapter.VerificationHandler, context.Context) error{
		"Submit": func(h *adapter.VerificationHandler, ctx context.Context) error {
			_, err := h.SubmitVerificationRequest(ctx, &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1"})
			return err
		},
		"ListMy": func(h *adapter.VerificationHandler, ctx context.Context) error {
			_, err := h.ListMyVerificationRequests(ctx, &cv1.ListMyVerificationRequestsRequest{})
			return err
		},
		"List": func(h *adapter.VerificationHandler, ctx context.Context) error {
			_, err := h.ListVerificationRequests(ctx, &cv1.ListVerificationRequestsRequest{})
			return err
		},
		"Approve": func(h *adapter.VerificationHandler, ctx context.Context) error {
			_, err := h.ApproveVerificationRequest(ctx, &cv1.ApproveVerificationRequestRequest{RequestId: requestID})
			return err
		},
		"Reject": func(h *adapter.VerificationHandler, ctx context.Context) error {
			_, err := h.RejectVerificationRequest(ctx, &cv1.RejectVerificationRequestRequest{RequestId: requestID, Reason: "x"})
			return err
		},
	}
	for _, tt := range []struct {
		rpc  string
		role string
	}{
		{"Submit", "admin"}, {"Submit", "applicant"},
		{"ListMy", "admin"}, {"ListMy", "applicant"},
		{"List", "employer"}, {"List", "applicant"},
		{"Approve", "employer"}, {"Approve", "applicant"},
		{"Reject", "employer"}, {"Reject", "applicant"},
	} {
		t.Run(tt.rpc+" as "+tt.role, func(t *testing.T) {
			h, _ := newHandler(t)
			wantCode(t, calls[tt.rpc](h, withCaller(employerID, tt.role)), codes.PermissionDenied)
		})
	}
}

func TestVerificationHandler_SubmitVerificationRequest(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *cv1.SubmitVerificationRequestRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "employer submits", req: &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1", Notes: "note"}, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return(nil, nil)
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "empty registration number", req: &cv1.SubmitVerificationRequestRequest{}, want: codes.InvalidArgument},
		{
			name: "pending request exists", req: &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1"}, want: codes.AlreadyExists,
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return([]domain.RequestStatus{domain.StatusPending}, nil)
			},
		},
		{
			name: "already approved", req: &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1"}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return([]domain.RequestStatus{domain.StatusApproved}, nil)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.SubmitVerificationRequest(asEmployer(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			r := resp.GetRequest()
			if r.GetRequestId() == "" || r.GetEmployerId() != employerID || r.GetRegistrationNumber() != "BT-1" ||
				r.GetStatus() != cv1.RequestStatus_REQUEST_STATUS_PENDING || r.GetReviewedBy() != "" ||
				!r.GetTimestamps().GetCreatedAt().AsTime().Equal(testTime) {
				t.Fatalf("unexpected request %v", r)
			}
		})
	}
}

func TestVerificationHandler_ListMyVerificationRequests(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *cv1.ListMyVerificationRequestsRequest
		setup func(d deps)
		want  codes.Code
		next  string
	}{
		{
			name: "nil request uses default page", want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().ListByEmployer(gomock.Any(), employerID, domain.Page{Limit: 20}).
					Return([]domain.VerificationRequest{stored(domain.StatusPending)}, int64(1), nil)
			},
		},
		{
			name: "page with more remaining", want: codes.OK, next: "1",
			req: &cv1.ListMyVerificationRequestsRequest{Pagination: &commonv1.PaginationRequest{PageSize: 1}},
			setup: func(d deps) {
				d.repo.EXPECT().ListByEmployer(gomock.Any(), employerID, domain.Page{Limit: 1}).
					Return([]domain.VerificationRequest{stored(domain.StatusPending)}, int64(3), nil)
			},
		},
		{name: "negative page size", want: codes.InvalidArgument,
			req: &cv1.ListMyVerificationRequestsRequest{Pagination: &commonv1.PaginationRequest{PageSize: -1}}},
		{name: "bad page token", want: codes.InvalidArgument,
			req: &cv1.ListMyVerificationRequestsRequest{Pagination: &commonv1.PaginationRequest{PageToken: "abc"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ListMyVerificationRequests(asEmployer(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && (len(resp.GetRequests()) != 1 || resp.GetPagination().GetNextPageToken() != tt.next ||
				resp.GetPagination().GetTotalCount() == 0) {
				t.Fatalf("unexpected response %v", resp)
			}
		})
	}
}

func TestVerificationHandler_ListVerificationRequests(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status cv1.RequestStatus
		setup  func(d deps)
		want   codes.Code
	}{
		{
			name: "all statuses", status: cv1.RequestStatus_REQUEST_STATUS_UNSPECIFIED, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusUnspecified, domain.Page{Limit: 20}).Return(nil, int64(0), nil)
			},
		},
		{
			name: "pending only", status: cv1.RequestStatus_REQUEST_STATUS_PENDING, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusPending, gomock.Any()).Return(nil, int64(0), nil)
			},
		},
		{
			name: "approved only", status: cv1.RequestStatus_REQUEST_STATUS_APPROVED, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusApproved, gomock.Any()).Return(nil, int64(0), nil)
			},
		},
		{
			name: "rejected only", status: cv1.RequestStatus_REQUEST_STATUS_REJECTED, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusRejected, gomock.Any()).Return(nil, int64(0), nil)
			},
		},
		{name: "unknown enum number", status: cv1.RequestStatus(7), want: codes.InvalidArgument},
		{name: "negative enum number", status: cv1.RequestStatus(-1), want: codes.InvalidArgument},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ListVerificationRequests(asAdmin(), &cv1.ListVerificationRequestsRequest{Status: tt.status})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && (resp.GetRequests() == nil || resp.GetPagination() == nil) {
				t.Fatalf("lists and pagination must be set, got %v", resp)
			}
		})
	}
}

func TestVerificationHandler_ApproveVerificationRequest(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *cv1.ApproveVerificationRequestRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "admin approves", req: &cv1.ApproveVerificationRequestRequest{RequestId: requestID}, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(testTime.Add(time.Hour))
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(returnDecided)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "request ID not a UUID", req: &cv1.ApproveVerificationRequestRequest{RequestId: "req-1"}, want: codes.InvalidArgument},
		{
			name: "unknown request", req: &cv1.ApproveVerificationRequestRequest{RequestId: requestID}, want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(domain.VerificationRequest{}, domain.ErrRequestNotFound)
			},
		},
		{
			name: "already decided", req: &cv1.ApproveVerificationRequestRequest{RequestId: requestID}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusRejected), nil)
				d.clock.EXPECT().Now().Return(testTime)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ApproveVerificationRequest(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				r := resp.GetRequest()
				if r.GetStatus() != cv1.RequestStatus_REQUEST_STATUS_APPROVED || r.GetReviewedBy() != adminID || r.GetRejectionReason() != "" {
					t.Fatalf("unexpected request %v", r)
				}
			}
		})
	}
}

func TestVerificationHandler_RejectVerificationRequest(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *cv1.RejectVerificationRequestRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "admin rejects with reason", req: &cv1.RejectVerificationRequestRequest{RequestId: requestID, Reason: "Expired licence"}, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(testTime.Add(time.Hour))
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(returnDecided)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "empty reason", req: &cv1.RejectVerificationRequestRequest{RequestId: requestID}, want: codes.InvalidArgument},
		{
			name: "already approved", req: &cv1.RejectVerificationRequestRequest{RequestId: requestID, Reason: "x"}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusApproved), nil)
				d.clock.EXPECT().Now().Return(testTime)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.RejectVerificationRequest(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				r := resp.GetRequest()
				if r.GetStatus() != cv1.RequestStatus_REQUEST_STATUS_REJECTED || r.GetReviewedBy() != adminID || r.GetRejectionReason() != "Expired licence" {
					t.Fatalf("unexpected request %v", r)
				}
			}
		})
	}
}

func TestVerificationHandler_HidesUnexpectedErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want codes.Code
	}{
		{"database failure", errDB, codes.Internal},
		{"canceled", context.Canceled, codes.Canceled},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(domain.VerificationRequest{}, tt.err)
			_, err := h.ApproveVerificationRequest(asAdmin(), &cv1.ApproveVerificationRequestRequest{RequestId: requestID})
			wantCode(t, err, tt.want)
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked driver detail: %v", err)
			}
		})
	}
}
