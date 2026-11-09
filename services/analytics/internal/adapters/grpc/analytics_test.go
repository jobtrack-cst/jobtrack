package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	anv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	adapter "github.com/jobtrack-cst/jobtrack/services/analytics/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/ports/mocks"
)

const (
	userID  = "11111111-1111-4111-8111-111111111111"
	adminID = "22222222-2222-4222-8222-222222222222"
)

var (
	errDB = errors.New("pq: secret SQL detail")
	bt    = time.FixedZone("BTT", 6*3600)
	start = time.Date(2026, 11, 2, 0, 0, 0, 500, bt)
	end   = time.Date(2026, 11, 9, 0, 0, 0, 0, bt)
	// The repository always gets the range in UTC.
	wantRange = domain.TimeRange{Start: start.UTC(), End: end.UTC()}
)

// Handlers are tested with the real app layer and a mocked repository.
func newHandlers(t *testing.T) (*adapter.AnalyticsHandler, *adapter.AnalyticsInternalHandler, *mocks.MockOutcomeRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutcomeRepository(ctrl)
	svc := app.NewAnalytics(repo)
	return adapter.NewAnalyticsHandler(svc), adapter.NewAnalyticsInternalHandler(svc), repo
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

func validRequest() *anv1.GetSummaryRequest {
	return &anv1.GetSummaryRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(end)}
}

// Requests that are invalid for both RPCs, as start and end pairs.
var invalidTimes = []struct {
	name       string
	start, end *timestamppb.Timestamp
}{
	{"no times", nil, nil},
	{"missing start", nil, timestamppb.New(end)},
	{"missing end", timestamppb.New(start), nil},
	{"start equal to end", timestamppb.New(start), timestamppb.New(start)},
	{"start after end", timestamppb.New(end), timestamppb.New(start)},
	{"start with negative nanos", &timestamppb.Timestamp{Seconds: 1, Nanos: -1}, timestamppb.New(end)},
	{"start nanos too large", &timestamppb.Timestamp{Seconds: 1, Nanos: 1e9}, timestamppb.New(end)},
	{"start before year 1", &timestamppb.Timestamp{Seconds: -62135596801}, timestamppb.New(end)},
	{"end after year 9999", timestamppb.New(start), &timestamppb.Timestamp{Seconds: 253402300800}},
}

func TestAnalyticsHandler_GetSummary_RejectsBadIdentity(t *testing.T) {
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
			h, _, _ := newHandlers(t) // no repository calls expected
			_, err := h.GetSummary(tt.ctx, validRequest())
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

// Role matrix: GetSummary, the only public RPC, is admin only.
func TestAnalyticsHandler_RoleMatrix(t *testing.T) {
	for _, tt := range []struct {
		role string
		want codes.Code
	}{
		{"applicant", codes.PermissionDenied},
		{"employer", codes.PermissionDenied},
		{"admin", codes.OK},
	} {
		t.Run("GetSummary as "+tt.role, func(t *testing.T) {
			h, _, repo := newHandlers(t)
			if tt.want == codes.OK {
				repo.EXPECT().CountEvents(gomock.Any(), wantRange).Return(nil, nil)
			}
			_, err := h.GetSummary(withCaller(userID, tt.role), validRequest())
			wantCode(t, err, tt.want)
		})
	}
}

// The role is checked before the input, so non-admins learn nothing else.
func TestAnalyticsHandler_RoleBeforeInput(t *testing.T) {
	for _, role := range []string{"applicant", "employer"} {
		t.Run(role, func(t *testing.T) {
			h, _, _ := newHandlers(t) // no repository calls expected
			if _, err := h.GetSummary(withCaller(userID, role), nil); status.Code(err) != codes.PermissionDenied {
				t.Fatalf("nil request: code=%v, want PermissionDenied", status.Code(err))
			}
			for _, tt := range invalidTimes {
				_, err := h.GetSummary(withCaller(userID, role), &anv1.GetSummaryRequest{StartTime: tt.start, EndTime: tt.end})
				if status.Code(err) != codes.PermissionDenied {
					t.Fatalf("%s: code=%v, want PermissionDenied", tt.name, status.Code(err))
				}
			}
		})
	}
}

func TestAnalyticsHandlers_RejectInvalidTimes(t *testing.T) {
	for _, tt := range invalidTimes {
		t.Run("GetSummary "+tt.name, func(t *testing.T) {
			h, _, _ := newHandlers(t) // no repository calls expected
			_, err := h.GetSummary(asAdmin(), &anv1.GetSummaryRequest{StartTime: tt.start, EndTime: tt.end})
			wantCode(t, err, codes.InvalidArgument)
		})
		t.Run("GetInternalSummary "+tt.name, func(t *testing.T) {
			_, h, _ := newHandlers(t)
			_, err := h.GetInternalSummary(context.Background(), &anv1.GetInternalSummaryRequest{StartTime: tt.start, EndTime: tt.end})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
	t.Run("nil requests", func(t *testing.T) {
		h, ih, _ := newHandlers(t)
		_, err := h.GetSummary(asAdmin(), nil)
		wantCode(t, err, codes.InvalidArgument)
		_, err = ih.GetInternalSummary(context.Background(), nil)
		wantCode(t, err, codes.InvalidArgument)
	})
}

func assertSummary(t *testing.T, got *anv1.Summary) {
	t.Helper()
	if got == nil {
		t.Fatal("summary is nil")
	}
	// The request times are echoed exactly, nanoseconds included.
	if !got.GetStartTime().AsTime().Equal(start) || !got.GetEndTime().AsTime().Equal(end) {
		t.Fatalf("times %v to %v, want %v to %v", got.GetStartTime().AsTime(), got.GetEndTime().AsTime(), start, end)
	}
	if got.GetJobsPublished() != 1 || got.GetJobsClosed() != 2 || got.GetApplicationsSubmitted() != 3 ||
		got.GetApplicationsWithdrawn() != 4 || got.GetOffersMade() != 5 || got.GetOffersAccepted() != 6 || got.GetOffersDeclined() != 7 {
		t.Fatalf("counters %v", got)
	}
	changes := got.GetStatusChanges()
	if len(changes) != 2 || changes[0].GetStatus() != "OFFERED" || changes[0].GetCount() != 8 ||
		changes[1].GetStatus() != "SHORTLISTED" || changes[1].GetCount() != 9 {
		t.Fatalf("status changes %v", changes)
	}
}

func allRows() []domain.EventCount {
	return []domain.EventCount{
		{EventType: domain.EventJobPublished, Count: 1},
		{EventType: domain.EventJobClosed, Count: 2},
		{EventType: domain.EventApplicationSubmitted, Count: 3},
		{EventType: domain.EventApplicationWithdrawn, Count: 4},
		{EventType: domain.EventOfferMade, Count: 5},
		{EventType: domain.EventOfferAccepted, Count: 6},
		{EventType: domain.EventOfferDeclined, Count: 7},
		{EventType: domain.EventStatusChanged, NewStatus: "SHORTLISTED", Count: 9},
		{EventType: domain.EventStatusChanged, NewStatus: "OFFERED", Count: 8},
	}
}

func TestAnalyticsHandlers_MapAllFields(t *testing.T) {
	t.Run("GetSummary", func(t *testing.T) {
		h, _, repo := newHandlers(t)
		repo.EXPECT().CountEvents(gomock.Any(), wantRange).Return(allRows(), nil)
		resp, err := h.GetSummary(asAdmin(), validRequest())
		wantCode(t, err, codes.OK)
		assertSummary(t, resp.GetSummary())
	})
	t.Run("GetInternalSummary", func(t *testing.T) {
		_, h, repo := newHandlers(t)
		repo.EXPECT().CountEvents(gomock.Any(), wantRange).Return(allRows(), nil)
		resp, err := h.GetInternalSummary(context.Background(), &anv1.GetInternalSummaryRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(end)})
		wantCode(t, err, codes.OK)
		assertSummary(t, resp.GetSummary())
	})
}

func TestAnalyticsHandlers_EmptyRange(t *testing.T) {
	h, _, repo := newHandlers(t)
	repo.EXPECT().CountEvents(gomock.Any(), gomock.Any()).Return(nil, nil)
	resp, err := h.GetSummary(asAdmin(), validRequest())
	wantCode(t, err, codes.OK)
	s := resp.GetSummary()
	if s == nil || s.GetJobsPublished() != 0 || s.GetOffersDeclined() != 0 || len(s.GetStatusChanges()) != 0 {
		t.Fatalf("got %v, want all zeros", s)
	}
}

// The internal RPC has no user: it needs no identity, and any forwarded
// identity is ignored, because the interceptor is what protects it.
func TestAnalyticsInternalHandler_IgnoresUserIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"applicant identity", withCaller(userID, "applicant")},
		{"malformed identity", withCaller("abc", "superuser")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, h, repo := newHandlers(t)
			repo.EXPECT().CountEvents(gomock.Any(), wantRange).Return(nil, nil)
			_, err := h.GetInternalSummary(tt.ctx, &anv1.GetInternalSummaryRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(end)})
			wantCode(t, err, codes.OK)
		})
	}
}

func TestAnalyticsHandler_UnixEpochIsASetTime(t *testing.T) {
	h, _, repo := newHandlers(t)
	repo.EXPECT().CountEvents(gomock.Any(), domain.TimeRange{Start: time.Unix(0, 0).UTC(), End: end.UTC()}).Return(nil, nil)
	_, err := h.GetSummary(asAdmin(), &anv1.GetSummaryRequest{StartTime: &timestamppb.Timestamp{}, EndTime: timestamppb.New(end)})
	wantCode(t, err, codes.OK)
}

func TestAnalyticsHandlers_HideUnexpectedErrors(t *testing.T) {
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
			h, ih, repo := newHandlers(t)
			repo.EXPECT().CountEvents(gomock.Any(), gomock.Any()).Return(nil, tt.err).Times(2)
			_, err := h.GetSummary(asAdmin(), validRequest())
			wantCode(t, err, tt.want)
			_, ierr := ih.GetInternalSummary(context.Background(), &anv1.GetInternalSummaryRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(end)})
			wantCode(t, ierr, tt.want)
			if strings.Contains(err.Error(), "secret") || strings.Contains(ierr.Error(), "secret") {
				t.Fatalf("error leaked driver detail: %v, %v", err, ierr)
			}
		})
	}
}
