package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	mv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/moderation/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/moderation/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports/mocks"
)

const (
	userID   = "11111111-1111-4111-8111-111111111111"
	adminID  = "22222222-2222-4222-8222-222222222222"
	reportID = "33333333-3333-4333-8333-333333333333"
	jobID    = "55555555-5555-4555-8555-555555555555"
)

var (
	errDB    = errors.New("pq: secret SQL detail")
	testTime = time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
)

type deps struct {
	repo  *mocks.MockReportRepository
	clock *mocks.MockClock
}

// Handlers are tested with the real app layer and mocked ports.
func newHandler(t *testing.T) (*adapter.ModerationHandler, deps) {
	ctrl := gomock.NewController(t)
	d := deps{repo: mocks.NewMockReportRepository(ctrl), clock: mocks.NewMockClock(ctrl)}
	return adapter.NewModerationHandler(app.NewReports(d.repo, d.clock)), d
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asApplicant() context.Context { return withCaller(userID, "applicant") }
func asAdmin() context.Context     { return withCaller(adminID, "admin") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func stored(s domain.ReportStatus) domain.Report {
	return domain.Report{
		ID: reportID, JobID: jobID, ReporterID: userID, Reason: "spam",
		Status: s, CreatedAt: testTime, UpdatedAt: testTime,
	}
}

func returnArg(_ context.Context, r domain.Report) (domain.Report, error) {
	return r, nil
}

// identityCases are the caller checks every RPC shares.
var identityCases = []struct {
	name string
	ctx  context.Context
}{
	{"no metadata", context.Background()},
	{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID))},
	{"user ID not a UUID", withCaller("abc", "admin")},
	{"unknown role", withCaller(adminID, "superuser")},
	{"uppercase role", withCaller(adminID, "Admin")},
}

func TestModerationHandler_RejectsBadIdentityOnEveryRPC(t *testing.T) {
	for _, tt := range identityCases {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.ReportJob(tt.ctx, &mv1.ReportJobRequest{JobId: jobID, Reason: "spam"})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.ListReports(tt.ctx, &mv1.ListReportsRequest{})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.DismissReport(tt.ctx, &mv1.DismissReportRequest{ReportId: reportID})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.RemoveJob(tt.ctx, &mv1.RemoveJobRequest{JobId: jobID, Reason: "fake"})
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

// Role matrix: every RPC with every role. ReportJob is open to any logged-in
// user; the other three are admin only.
func TestModerationHandler_RoleMatrix(t *testing.T) {
	type call func(*adapter.ModerationHandler, context.Context) error
	calls := map[string]call{
		"ReportJob": func(h *adapter.ModerationHandler, ctx context.Context) error {
			_, err := h.ReportJob(ctx, &mv1.ReportJobRequest{JobId: jobID, Reason: "spam"})
			return err
		},
		"ListReports": func(h *adapter.ModerationHandler, ctx context.Context) error {
			_, err := h.ListReports(ctx, &mv1.ListReportsRequest{})
			return err
		},
		"DismissReport": func(h *adapter.ModerationHandler, ctx context.Context) error {
			_, err := h.DismissReport(ctx, &mv1.DismissReportRequest{ReportId: reportID})
			return err
		},
		"RemoveJob": func(h *adapter.ModerationHandler, ctx context.Context) error {
			_, err := h.RemoveJob(ctx, &mv1.RemoveJobRequest{JobId: jobID, Reason: "fake"})
			return err
		},
	}
	// allowed sets up the repository for a call that passes the role check.
	allowed := map[string]func(d deps){
		"ReportJob": func(d deps) {
			d.clock.EXPECT().Now().Return(testTime)
			d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
		},
		"ListReports": func(d deps) {
			d.repo.EXPECT().List(gomock.Any(), domain.StatusUnspecified, domain.Page{Limit: 20}).Return(nil, int64(0), nil)
		},
		"DismissReport": func(d deps) {
			d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(stored(domain.StatusOpen), nil)
			d.clock.EXPECT().Now().Return(testTime)
			d.repo.EXPECT().Dismiss(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
		},
		"RemoveJob": func(d deps) {
			d.clock.EXPECT().Now().Return(testTime)
			d.repo.EXPECT().RemoveJob(gomock.Any(), gomock.Any()).Return(nil)
		},
	}
	for _, tt := range []struct {
		rpc  string
		role string
		want codes.Code
	}{
		{"ReportJob", "applicant", codes.OK}, {"ReportJob", "employer", codes.OK}, {"ReportJob", "admin", codes.OK},
		{"ListReports", "applicant", codes.PermissionDenied}, {"ListReports", "employer", codes.PermissionDenied}, {"ListReports", "admin", codes.OK},
		{"DismissReport", "applicant", codes.PermissionDenied}, {"DismissReport", "employer", codes.PermissionDenied}, {"DismissReport", "admin", codes.OK},
		{"RemoveJob", "applicant", codes.PermissionDenied}, {"RemoveJob", "employer", codes.PermissionDenied}, {"RemoveJob", "admin", codes.OK},
	} {
		t.Run(tt.rpc+" as "+tt.role, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.want == codes.OK {
				allowed[tt.rpc](d)
			}
			wantCode(t, calls[tt.rpc](h, withCaller(userID, tt.role)), tt.want)
		})
	}
}

// The role is checked before the input, so non-admins learn nothing else.
func TestModerationHandler_RoleBeforeInput(t *testing.T) {
	for _, role := range []string{"applicant", "employer"} {
		t.Run(role, func(t *testing.T) {
			h, _ := newHandler(t)
			ctx := withCaller(userID, role)
			_, err := h.ListReports(ctx, &mv1.ListReportsRequest{Status: mv1.ReportStatus(9), Pagination: &commonv1.PaginationRequest{PageSize: -1}})
			wantCode(t, err, codes.PermissionDenied)
			_, err = h.DismissReport(ctx, nil)
			wantCode(t, err, codes.PermissionDenied)
			_, err = h.RemoveJob(ctx, &mv1.RemoveJobRequest{JobId: "bad"})
			wantCode(t, err, codes.PermissionDenied)
		})
	}
}

func TestModerationHandler_ReportJob(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *mv1.ReportJobRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "applicant reports", req: &mv1.ReportJobRequest{JobId: strings.ToUpper(jobID), Reason: " spam "}, want: codes.OK,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "job ID not a UUID", req: &mv1.ReportJobRequest{JobId: "job-1", Reason: "spam"}, want: codes.InvalidArgument},
		{name: "empty reason", req: &mv1.ReportJobRequest{JobId: jobID}, want: codes.InvalidArgument},
		{name: "reason one over the maximum", req: &mv1.ReportJobRequest{JobId: jobID, Reason: strings.Repeat("a", 1001)}, want: codes.InvalidArgument},
		{
			name: "same user while OPEN", req: &mv1.ReportJobRequest{JobId: jobID, Reason: "spam"}, want: codes.AlreadyExists,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Report{}, domain.ErrOpenReportExists)
			},
		},
		{
			name: "job already removed", req: &mv1.ReportJobRequest{JobId: jobID, Reason: "spam"}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Report{}, domain.ErrJobAlreadyRemoved)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ReportJob(asApplicant(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			r := resp.GetReport()
			if r.GetReportId() == "" || r.GetJobId() != jobID || r.GetReporterId() != userID || r.GetReason() != "spam" ||
				r.GetStatus() != mv1.ReportStatus_REPORT_STATUS_OPEN ||
				!r.GetTimestamps().GetCreatedAt().AsTime().Equal(testTime) || !r.GetTimestamps().GetUpdatedAt().AsTime().Equal(testTime) {
				t.Fatalf("unexpected report %v", r)
			}
		})
	}
}

func TestModerationHandler_ListReports(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *mv1.ListReportsRequest
		setup func(d deps)
		want  codes.Code
		count int
		next  string
	}{
		{
			name: "nil request uses default page and all statuses", want: codes.OK, count: 1,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusUnspecified, domain.Page{Limit: 20}).
					Return([]domain.Report{stored(domain.StatusOpen)}, int64(1), nil)
			},
		},
		{
			name: "open filter with more remaining", want: codes.OK, count: 1, next: "1",
			req: &mv1.ListReportsRequest{Status: mv1.ReportStatus_REPORT_STATUS_OPEN, Pagination: &commonv1.PaginationRequest{PageSize: 1}},
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusOpen, domain.Page{Limit: 1}).
					Return([]domain.Report{stored(domain.StatusOpen)}, int64(3), nil)
			},
		},
		{
			name: "dismissed filter", want: codes.OK,
			req: &mv1.ListReportsRequest{Status: mv1.ReportStatus_REPORT_STATUS_DISMISSED},
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusDismissed, gomock.Any()).Return(nil, int64(0), nil)
			},
		},
		{
			name: "actioned filter", want: codes.OK,
			req: &mv1.ListReportsRequest{Status: mv1.ReportStatus_REPORT_STATUS_ACTIONED},
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusActioned, gomock.Any()).Return(nil, int64(0), nil)
			},
		},
		{name: "unknown enum number", req: &mv1.ListReportsRequest{Status: mv1.ReportStatus(4)}, want: codes.InvalidArgument},
		{name: "negative enum number", req: &mv1.ListReportsRequest{Status: mv1.ReportStatus(-1)}, want: codes.InvalidArgument},
		{name: "negative page size", want: codes.InvalidArgument,
			req: &mv1.ListReportsRequest{Pagination: &commonv1.PaginationRequest{PageSize: -1}}},
		{name: "bad page token", want: codes.InvalidArgument,
			req: &mv1.ListReportsRequest{Pagination: &commonv1.PaginationRequest{PageToken: "abc"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ListReports(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			if resp.GetReports() == nil || resp.GetPagination() == nil || len(resp.GetReports()) != tt.count ||
				resp.GetPagination().GetNextPageToken() != tt.next {
				t.Fatalf("unexpected response %v", resp)
			}
		})
	}
}

func TestModerationHandler_DismissReport(t *testing.T) {
	later := testTime.Add(time.Hour)
	for _, tt := range []struct {
		name  string
		req   *mv1.DismissReportRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "admin dismisses", req: &mv1.DismissReportRequest{ReportId: reportID}, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(stored(domain.StatusOpen), nil)
				d.clock.EXPECT().Now().Return(later)
				d.repo.EXPECT().Dismiss(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "report ID not a UUID", req: &mv1.DismissReportRequest{ReportId: "report-1"}, want: codes.InvalidArgument},
		{
			name: "unknown report", req: &mv1.DismissReportRequest{ReportId: reportID}, want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(domain.Report{}, domain.ErrReportNotFound)
			},
		},
		{
			name: "already dismissed", req: &mv1.DismissReportRequest{ReportId: reportID}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(stored(domain.StatusDismissed), nil)
				d.clock.EXPECT().Now().Return(later)
			},
		},
		{
			name: "already actioned", req: &mv1.DismissReportRequest{ReportId: reportID}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(stored(domain.StatusActioned), nil)
				d.clock.EXPECT().Now().Return(later)
			},
		},
		{
			name: "another admin closed it first", req: &mv1.DismissReportRequest{ReportId: reportID}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(stored(domain.StatusOpen), nil)
				d.clock.EXPECT().Now().Return(later)
				d.repo.EXPECT().Dismiss(gomock.Any(), gomock.Any()).Return(domain.Report{}, domain.ErrReportNotOpen)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.DismissReport(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				r := resp.GetReport()
				if r.GetStatus() != mv1.ReportStatus_REPORT_STATUS_DISMISSED || r.GetReportId() != reportID ||
					!r.GetTimestamps().GetUpdatedAt().AsTime().Equal(later) {
					t.Fatalf("unexpected report %v", r)
				}
			}
		})
	}
}

func TestModerationHandler_RemoveJob(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *mv1.RemoveJobRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "admin removes", req: &mv1.RemoveJobRequest{JobId: strings.ToUpper(jobID), Reason: "fake company"}, want: codes.OK,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().RemoveJob(gomock.Any(), domain.Removal{JobID: jobID, Reason: "fake company", RemovedBy: adminID, RemovedAt: testTime}).Return(nil)
			},
		},
		{name: "nil request", want: codes.InvalidArgument},
		{name: "job ID not a UUID", req: &mv1.RemoveJobRequest{JobId: "job-1", Reason: "x"}, want: codes.InvalidArgument},
		{name: "empty reason", req: &mv1.RemoveJobRequest{JobId: jobID}, want: codes.InvalidArgument},
		{name: "reason one over the maximum", req: &mv1.RemoveJobRequest{JobId: jobID, Reason: strings.Repeat("a", 501)}, want: codes.InvalidArgument},
		{
			name: "second takedown", req: &mv1.RemoveJobRequest{JobId: jobID, Reason: "x"}, want: codes.FailedPrecondition,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().RemoveJob(gomock.Any(), gomock.Any()).Return(domain.ErrJobAlreadyRemoved)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.RemoveJob(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && resp.GetJobId() != jobID {
				t.Fatalf("job_id=%q, want %q", resp.GetJobId(), jobID)
			}
		})
	}
}

func TestModerationHandler_HidesUnexpectedErrors(t *testing.T) {
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
			d.repo.EXPECT().GetByID(gomock.Any(), reportID).Return(domain.Report{}, tt.err)
			_, err := h.DismissReport(asAdmin(), &mv1.DismissReportRequest{ReportId: reportID})
			wantCode(t, err, tt.want)
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked driver detail: %v", err)
			}
		})
	}
}
