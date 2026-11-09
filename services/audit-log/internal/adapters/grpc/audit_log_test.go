package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	alv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/audit_log/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	adapter "github.com/jobtrack-cst/jobtrack/services/audit-log/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports/mocks"
)

const (
	userID  = "11111111-1111-4111-8111-111111111111"
	adminID = "22222222-2222-4222-8222-222222222222"
	eventID = "33333333-3333-4333-8333-333333333333"
)

var (
	errDB    = errors.New("pq: secret SQL detail")
	testTime = time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
)

// Handlers are tested with the real app layer and a mocked repository.
func newHandler(t *testing.T) (*adapter.AuditLogHandler, *mocks.MockEntryRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEntryRepository(ctrl)
	return adapter.NewAuditLogHandler(app.NewAuditLog(repo)), repo
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asAdmin() context.Context { return withCaller(adminID, "admin") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func stored() domain.Entry {
	return domain.Entry{
		EventID: eventID, EventType: "job.published", Producer: "job-posting",
		OccurredAt: testTime, DataJSON: `{ "job_id": "x" }`,
	}
}

func ptr(t time.Time) *time.Time { return &t }

var defaultPage = domain.Page{Limit: 20}

func TestAuditLogHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", adminID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "admin"))},
		{"user ID not a UUID", withCaller("abc", "admin")},
		{"unknown role", withCaller(adminID, "superuser")},
		{"uppercase role", withCaller(adminID, "Admin")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t) // no repository calls expected
			_, err := h.ListAuditEntries(tt.ctx, &alv1.ListAuditEntriesRequest{})
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

// Role matrix: ListAuditEntries, the only RPC, is admin only.
func TestAuditLogHandler_RoleMatrix(t *testing.T) {
	for _, tt := range []struct {
		role string
		want codes.Code
	}{
		{"applicant", codes.PermissionDenied},
		{"employer", codes.PermissionDenied},
		{"admin", codes.OK},
	} {
		t.Run("ListAuditEntries as "+tt.role, func(t *testing.T) {
			h, repo := newHandler(t)
			if tt.want == codes.OK {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{}, defaultPage).Return(nil, int64(0), nil)
			}
			_, err := h.ListAuditEntries(withCaller(userID, tt.role), &alv1.ListAuditEntriesRequest{})
			wantCode(t, err, tt.want)
		})
	}
}

// The role is checked before the input, so non-admins learn nothing else.
func TestAuditLogHandler_RoleBeforeInput(t *testing.T) {
	bad := []*alv1.ListAuditEntriesRequest{
		nil,
		{EventType: "BAD"},
		{StartTime: &timestamppb.Timestamp{Nanos: -1}},
		{EndTime: &timestamppb.Timestamp{Seconds: 1 << 40}},
		{StartTime: timestamppb.New(testTime), EndTime: timestamppb.New(testTime)},
		{Pagination: &commonv1.PaginationRequest{PageSize: -1, PageToken: "x"}},
	}
	for _, role := range []string{"applicant", "employer"} {
		t.Run(role, func(t *testing.T) {
			h, _ := newHandler(t) // no repository calls expected
			for i, req := range bad {
				_, err := h.ListAuditEntries(withCaller(userID, role), req)
				if status.Code(err) != codes.PermissionDenied {
					t.Fatalf("request %d: code=%v, want PermissionDenied", i, status.Code(err))
				}
			}
		})
	}
}

func TestAuditLogHandler_ListAuditEntries(t *testing.T) {
	bt := time.FixedZone("BTT", 6*3600)
	start := time.Date(2026, 11, 8, 9, 0, 0, 0, bt)
	end := start.Add(time.Hour)
	for _, tt := range []struct {
		name  string
		req   *alv1.ListAuditEntriesRequest
		setup func(repo *mocks.MockEntryRepository)
		want  codes.Code
		count int
		next  string
		total int64
	}{
		{
			name: "nil request uses no filters and the default page", want: codes.OK, count: 1, total: 1,
			setup: func(repo *mocks.MockEntryRepository) {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{}, defaultPage).
					Return([]domain.Entry{stored()}, int64(1), nil)
			},
		},
		{
			name: "all filters are passed in UTC, with more pages", want: codes.OK, count: 1, next: "3", total: 9,
			req: &alv1.ListAuditEntriesRequest{
				EventType:  "job.published",
				StartTime:  timestamppb.New(start),
				EndTime:    timestamppb.New(end),
				Pagination: &commonv1.PaginationRequest{PageSize: 1, PageToken: "2"},
			},
			setup: func(repo *mocks.MockEntryRepository) {
				want := domain.ListFilter{EventType: "job.published", Start: ptr(start.UTC()), End: ptr(end.UTC())}
				repo.EXPECT().ListEntries(gomock.Any(), want, domain.Page{Limit: 1, Offset: 2}).
					Return([]domain.Entry{stored()}, int64(9), nil)
			},
		},
		{
			name: "start only", want: codes.OK,
			req: &alv1.ListAuditEntriesRequest{StartTime: timestamppb.New(start)},
			setup: func(repo *mocks.MockEntryRepository) {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{Start: ptr(start.UTC())}, defaultPage).Return(nil, int64(0), nil)
			},
		},
		{
			name: "end only", want: codes.OK,
			req: &alv1.ListAuditEntriesRequest{EndTime: timestamppb.New(end)},
			setup: func(repo *mocks.MockEntryRepository) {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{End: ptr(end.UTC())}, defaultPage).Return(nil, int64(0), nil)
			},
		},
		{
			name: "Unix epoch is a set filter, not an unset one", want: codes.OK,
			req: &alv1.ListAuditEntriesRequest{StartTime: &timestamppb.Timestamp{}},
			setup: func(repo *mocks.MockEntryRepository) {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{Start: ptr(time.Unix(0, 0).UTC())}, defaultPage).Return(nil, int64(0), nil)
			},
		},
		{
			name: "page size above the maximum is capped", want: codes.OK,
			req: &alv1.ListAuditEntriesRequest{Pagination: &commonv1.PaginationRequest{PageSize: 101}},
			setup: func(repo *mocks.MockEntryRepository) {
				repo.EXPECT().ListEntries(gomock.Any(), domain.ListFilter{}, domain.Page{Limit: 100}).Return(nil, int64(0), nil)
			},
		},
		{name: "start equal to end", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(start)}},
		{name: "start after end", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{StartTime: timestamppb.New(end), EndTime: timestamppb.New(start)}},
		{name: "start with negative nanos", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{StartTime: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}}},
		{name: "end after year 9999", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{EndTime: &timestamppb.Timestamp{Seconds: 253402300800}}},
		{name: "uppercase event type", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{EventType: "Job.Published"}},
		{name: "event type 101 characters", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{EventType: "a." + strings.Repeat("b", 99)}},
		{name: "negative page size", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{Pagination: &commonv1.PaginationRequest{PageSize: -1}}},
		{name: "bad page token", want: codes.InvalidArgument,
			req: &alv1.ListAuditEntriesRequest{Pagination: &commonv1.PaginationRequest{PageToken: "abc"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := newHandler(t)
			if tt.setup != nil {
				tt.setup(repo)
			}
			resp, err := h.ListAuditEntries(asAdmin(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			if resp.GetEntries() == nil || resp.GetPagination() == nil || len(resp.GetEntries()) != tt.count ||
				resp.GetPagination().GetNextPageToken() != tt.next || resp.GetPagination().GetTotalCount() != tt.total {
				t.Fatalf("unexpected response %v", resp)
			}
		})
	}
}

func TestAuditLogHandler_MapsEntryFields(t *testing.T) {
	h, repo := newHandler(t)
	repo.EXPECT().ListEntries(gomock.Any(), gomock.Any(), gomock.Any()).Return([]domain.Entry{stored()}, int64(1), nil)
	resp, err := h.ListAuditEntries(asAdmin(), nil)
	wantCode(t, err, codes.OK)
	got := resp.GetEntries()[0]
	want := stored()
	if got.GetEventId() != want.EventID || got.GetEventType() != want.EventType || got.GetProducer() != want.Producer ||
		!got.GetOccurredAt().AsTime().Equal(want.OccurredAt) || got.GetDataJson() != want.DataJSON {
		t.Fatalf("got %v, want %+v", got, want)
	}
}

func TestAuditLogHandler_HidesUnexpectedErrors(t *testing.T) {
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
			h, repo := newHandler(t)
			repo.EXPECT().ListEntries(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), tt.err)
			_, err := h.ListAuditEntries(asAdmin(), &alv1.ListAuditEntriesRequest{})
			wantCode(t, err, tt.want)
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked driver detail: %v", err)
			}
		})
	}
}
