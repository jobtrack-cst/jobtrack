package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	adapter "github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	otherID    = "22222222-2222-4222-8222-222222222222"
	reason     = "missing licence"
)

var (
	errDB    = errors.New("pq: secret SQL detail")
	testTime = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	rejected = domain.CompanyProfile{
		EmployerID: employerID, CompanyName: "Druk Tech", Description: "Software", Website: "https://druk.bt", Location: "Thimphu",
		VerificationStatus: domain.VerificationRejected, RejectionReason: reason, CreatedAt: testTime, UpdatedAt: testTime,
	}
	createReq = &employerv1.CreateCompanyProfileRequest{CompanyName: "Druk Tech", Description: "Software", Website: "https://druk.bt", Location: "Thimphu"}
	updateReq = &employerv1.UpdateCompanyProfileRequest{CompanyName: "Druk Tech Pvt", Location: "Paro"}
)

type deps struct {
	repo  *mocks.MockProfileRepository
	clock *mocks.MockClock
}

// Handlers are tested with the real app layer and mocked ports.
func newHandler(t *testing.T) (*adapter.EmployerHandler, deps) {
	ctrl := gomock.NewController(t)
	d := deps{repo: mocks.NewMockProfileRepository(ctrl), clock: mocks.NewMockClock(ctrl)}
	return adapter.NewEmployerHandler(app.NewProfiles(d.repo, d.clock)), d
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

// identityCases are the caller checks every RPC shares.
var identityCases = []struct {
	name string
	ctx  context.Context
	want codes.Code
}{
	{"no metadata", context.Background(), codes.Unauthenticated},
	{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", employerID)), codes.Unauthenticated},
	{"user ID not a UUID", withCaller("abc", "employer"), codes.Unauthenticated},
	{"unknown role", withCaller(employerID, "superuser"), codes.Unauthenticated},
}

func TestEmployerHandler_RejectsBadIdentityOnEveryRPC(t *testing.T) {
	for _, tt := range identityCases {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.CreateCompanyProfile(tt.ctx, createReq)
			wantCode(t, err, tt.want)
			_, err = h.GetMyCompanyProfile(tt.ctx, &employerv1.GetMyCompanyProfileRequest{})
			wantCode(t, err, tt.want)
			_, err = h.UpdateCompanyProfile(tt.ctx, updateReq)
			wantCode(t, err, tt.want)
			_, err = h.GetCompanyProfile(tt.ctx, &employerv1.GetCompanyProfileRequest{EmployerId: employerID})
			wantCode(t, err, tt.want)
		})
	}
}

func TestEmployerHandler_CreateCompanyProfile(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *employerv1.CreateCompanyProfileRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "employer creates unverified profile", ctx: withCaller(employerID, "employer"), req: createReq, want: codes.OK,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) { return p, nil })
			},
		},
		{name: "applicant is denied", ctx: withCaller(employerID, "applicant"), req: createReq, want: codes.PermissionDenied},
		{name: "admin is denied", ctx: withCaller(employerID, "admin"), req: createReq, want: codes.PermissionDenied},
		{name: "nil request", ctx: withCaller(employerID, "employer"), want: codes.InvalidArgument},
		{name: "empty company name", ctx: withCaller(employerID, "employer"), req: &employerv1.CreateCompanyProfileRequest{}, want: codes.InvalidArgument},
		{
			name: "second profile", ctx: withCaller(employerID, "employer"), req: createReq, want: codes.AlreadyExists,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(testTime)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.CompanyProfile{}, domain.ErrProfileExists)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.CreateCompanyProfile(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			p := resp.GetProfile()
			if p.GetEmployerId() != employerID || p.GetCompanyName() != "Druk Tech" ||
				p.GetVerificationStatus() != employerv1.VerificationStatus_VERIFICATION_STATUS_UNVERIFIED {
				t.Fatalf("unexpected profile %v", p)
			}
		})
	}
}

func TestEmployerHandler_GetMyCompanyProfile(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "employer gets own profile", ctx: withCaller(employerID, "employer"), want: codes.OK,
			setup: func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(rejected, nil) },
		},
		{name: "applicant is denied", ctx: withCaller(employerID, "applicant"), want: codes.PermissionDenied},
		{
			name: "no profile yet", ctx: withCaller(employerID, "employer"), want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.GetMyCompanyProfile(tt.ctx, &employerv1.GetMyCompanyProfileRequest{})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && resp.GetProfile().GetRejectionReason() != reason {
				t.Fatalf("owner should see rejection reason, got %v", resp.GetProfile())
			}
		})
	}
}

func TestEmployerHandler_UpdateCompanyProfile(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *employerv1.UpdateCompanyProfileRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "owner updates and keeps verification", ctx: withCaller(employerID, "employer"), req: updateReq, want: codes.OK,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(rejected, nil)
				d.clock.EXPECT().Now().Return(testTime.Add(time.Hour))
				d.repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, p domain.CompanyProfile) (domain.CompanyProfile, error) { return p, nil })
			},
		},
		{name: "applicant is denied", ctx: withCaller(employerID, "applicant"), req: updateReq, want: codes.PermissionDenied},
		{name: "admin is denied", ctx: withCaller(employerID, "admin"), req: updateReq, want: codes.PermissionDenied},
		{name: "nil request", ctx: withCaller(employerID, "employer"), want: codes.InvalidArgument},
		{name: "bad website", ctx: withCaller(employerID, "employer"),
			req: &employerv1.UpdateCompanyProfileRequest{CompanyName: "D", Website: "ftp://druk.bt"}, want: codes.InvalidArgument},
		{
			name: "no profile to update", ctx: withCaller(otherID, "employer"), req: updateReq, want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), otherID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.UpdateCompanyProfile(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			p := resp.GetProfile()
			if p.GetCompanyName() != "Druk Tech Pvt" || p.GetLocation() != "Paro" ||
				p.GetVerificationStatus() != employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED ||
				!p.GetTimestamps().GetUpdatedAt().AsTime().Equal(testTime.Add(time.Hour)) {
				t.Fatalf("unexpected profile %v", p)
			}
		})
	}
}

func TestEmployerHandler_GetCompanyProfile(t *testing.T) {
	found := func(d deps) { d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(rejected, nil) }
	for _, tt := range []struct {
		name       string
		ctx        context.Context
		id         string
		setup      func(d deps)
		want       codes.Code
		wantReason string
	}{
		{name: "owner sees reason", ctx: withCaller(employerID, "employer"), id: employerID, setup: found, want: codes.OK, wantReason: reason},
		{name: "admin sees reason", ctx: withCaller(otherID, "admin"), id: employerID, setup: found, want: codes.OK, wantReason: reason},
		{name: "applicant does not see reason", ctx: withCaller(otherID, "applicant"), id: employerID, setup: found, want: codes.OK},
		// job-posting forwards the publishing employer's identity in Sprint 2.
		{name: "forwarded identity of another employer", ctx: withCaller(otherID, "employer"), id: employerID, setup: found, want: codes.OK},
		{name: "ID not a UUID", ctx: withCaller(otherID, "applicant"), id: "abc", want: codes.InvalidArgument},
		{name: "nil request has empty ID", ctx: withCaller(otherID, "applicant"), want: codes.InvalidArgument},
		{
			name: "unknown employer", ctx: withCaller(otherID, "applicant"), id: otherID, want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetByEmployerID(gomock.Any(), otherID).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			var req *employerv1.GetCompanyProfileRequest
			if tt.id != "" {
				req = &employerv1.GetCompanyProfileRequest{EmployerId: tt.id}
			}
			resp, err := h.GetCompanyProfile(tt.ctx, req)
			if req == nil {
				// A nil request is rejected before the app layer is called.
				wantCode(t, err, codes.InvalidArgument)
				return
			}
			wantCode(t, err, tt.want)
			if tt.want != codes.OK {
				return
			}
			p := resp.GetProfile()
			if p.GetEmployerId() != employerID || p.GetRejectionReason() != tt.wantReason ||
				p.GetVerificationStatus() != employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED ||
				!p.GetTimestamps().GetCreatedAt().AsTime().Equal(testTime) {
				t.Fatalf("unexpected profile %v", p)
			}
		})
	}
}

func TestEmployerHandler_HidesUnexpectedErrors(t *testing.T) {
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
			d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(domain.CompanyProfile{}, tt.err)
			_, err := h.GetCompanyProfile(withCaller(otherID, "applicant"), &employerv1.GetCompanyProfileRequest{EmployerId: employerID})
			wantCode(t, err, tt.want)
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("driver detail leaked: %v", err)
			}
		})
	}
}
