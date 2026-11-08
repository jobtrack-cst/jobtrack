package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports/mocks"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	adminID    = "22222222-2222-4222-8222-222222222222"
	requestID  = "33333333-3333-4333-8333-333333333333"
	otherID    = "44444444-4444-4444-8444-444444444444"
)

var (
	errDB     = errors.New("database down")
	created   = time.Date(2026, 11, 8, 4, 0, 0, 0, time.UTC)
	now       = time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
	employer  = domain.Caller{UserID: employerID, Role: domain.RoleEmployer}
	admin     = domain.Caller{UserID: adminID, Role: domain.RoleAdmin}
	applicant = domain.Caller{UserID: otherID, Role: domain.RoleApplicant}
	noRole    = domain.Caller{UserID: otherID}
)

type deps struct {
	repo  *mocks.MockRequestRepository
	clock *mocks.MockClock
}

func newRequests(t *testing.T) (*app.Requests, deps) {
	ctrl := gomock.NewController(t)
	d := deps{repo: mocks.NewMockRequestRepository(ctrl), clock: mocks.NewMockClock(ctrl)}
	return app.NewRequests(d.repo, d.clock), d
}

func stored(status domain.RequestStatus) domain.VerificationRequest {
	return domain.VerificationRequest{
		ID: requestID, EmployerID: employerID, RegistrationNumber: "BT-1", Notes: "note",
		Status: status, CreatedAt: created, UpdatedAt: created,
	}
}

// returnArg makes Create and Decide behave like a repository that stores what it is given.
func returnArg(_ context.Context, r domain.VerificationRequest) (domain.VerificationRequest, error) {
	return r, nil
}

func TestRequests_Submit(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		reg     string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "employer submits first request", caller: employer, reg: " BT-1 ",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return(nil, nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{
			name: "employer submits again after a rejection", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return([]domain.RequestStatus{domain.StatusRejected}, nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "admin is denied", caller: admin, reg: "BT-1", wantErr: domain.ErrEmployerRoleRequired},
		{name: "applicant is denied", caller: applicant, reg: "BT-1", wantErr: domain.ErrEmployerRoleRequired},
		{name: "caller without role is denied", caller: noRole, reg: "BT-1", wantErr: domain.ErrEmployerRoleRequired},
		{name: "role is checked before input", caller: admin, reg: "", wantErr: domain.ErrEmployerRoleRequired},
		{name: "empty registration number", caller: employer, reg: "  ", wantErr: domain.ErrInvalidRegistrationNumber},
		{
			name: "pending request already exists", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return([]domain.RequestStatus{domain.StatusPending}, nil)
			},
			wantErr: domain.ErrPendingRequestExists,
		},
		{
			name: "employer already verified", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return([]domain.RequestStatus{domain.StatusApproved}, nil)
			},
			wantErr: domain.ErrAlreadyVerified,
		},
		{
			name: "concurrent submit caught by the repository", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return(nil, nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.VerificationRequest{}, domain.ErrPendingRequestExists)
			},
			wantErr: domain.ErrPendingRequestExists,
		},
		{
			name: "status lookup error is passed through", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return(nil, errDB)
			},
			wantErr: errDB,
		},
		{
			name: "create error is passed through", caller: employer, reg: "BT-1",
			setup: func(d deps) {
				d.repo.EXPECT().StatusesByEmployer(gomock.Any(), employerID).Return(nil, nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.VerificationRequest{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newRequests(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := s.Submit(context.Background(), tt.caller, tt.reg, " note ")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if _, err := uuid.Parse(got.ID); err != nil {
				t.Errorf("ID=%q is not a UUID", got.ID)
			}
			want := domain.VerificationRequest{
				ID: got.ID, EmployerID: employerID, RegistrationNumber: "BT-1", Notes: "note",
				Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now,
			}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestRequests_ListMine(t *testing.T) {
	page := []domain.VerificationRequest{stored(domain.StatusRejected), stored(domain.StatusPending)}
	for _, tt := range []struct {
		name      string
		caller    domain.Caller
		size      int32
		token     string
		setup     func(d deps)
		wantErr   error
		wantNext  string
		wantTotal int64
	}{
		{
			name: "employer lists own requests with default page", caller: employer,
			setup: func(d deps) {
				d.repo.EXPECT().ListByEmployer(gomock.Any(), employerID, domain.Page{Limit: 20}).Return(page, int64(2), nil)
			},
			wantTotal: 2,
		},
		{
			name: "next page token when more remain", caller: employer, size: 2, token: "2",
			setup: func(d deps) {
				d.repo.EXPECT().ListByEmployer(gomock.Any(), employerID, domain.Page{Limit: 2, Offset: 2}).Return(page, int64(5), nil)
			},
			wantTotal: 5, wantNext: "4",
		},
		{name: "admin is denied", caller: admin, wantErr: domain.ErrEmployerRoleRequired},
		{name: "applicant is denied", caller: applicant, wantErr: domain.ErrEmployerRoleRequired},
		{name: "negative page size", caller: employer, size: -1, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: employer, token: "abc", wantErr: domain.ErrInvalidPageToken},
		{
			name: "repository error is passed through", caller: employer,
			setup: func(d deps) {
				d.repo.EXPECT().ListByEmployer(gomock.Any(), employerID, gomock.Any()).Return(nil, int64(0), errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newRequests(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := s.ListMine(context.Background(), tt.caller, tt.size, tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if len(got.Requests) != len(page) || got.TotalCount != tt.wantTotal || got.NextPageToken != tt.wantNext {
				t.Errorf("got %d items, total %d, next %q", len(got.Requests), got.TotalCount, got.NextPageToken)
			}
		})
	}
}

func TestRequests_List(t *testing.T) {
	page := []domain.VerificationRequest{stored(domain.StatusPending)}
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		status  domain.RequestStatus
		size    int32
		token   string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "admin lists all statuses", caller: admin, status: domain.StatusUnspecified,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusUnspecified, domain.Page{Limit: 20}).Return(page, int64(1), nil)
			},
		},
		{
			name: "admin lists pending only", caller: admin, status: domain.StatusPending, size: 500,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), domain.StatusPending, domain.Page{Limit: 100}).Return(page, int64(1), nil)
			},
		},
		{name: "employer is denied", caller: employer, wantErr: domain.ErrAdminRoleRequired},
		{name: "applicant is denied", caller: applicant, wantErr: domain.ErrAdminRoleRequired},
		{name: "caller without role is denied", caller: noRole, wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the filter", caller: employer, status: 99, wantErr: domain.ErrAdminRoleRequired},
		{name: "unknown status filter", caller: admin, status: 99, wantErr: domain.ErrInvalidStatusFilter},
		{name: "negative page size", caller: admin, size: -5, wantErr: domain.ErrInvalidPageSize},
		{name: "bad page token", caller: admin, token: "+1", wantErr: domain.ErrInvalidPageToken},
		{
			name: "repository error is passed through", caller: admin,
			setup: func(d deps) {
				d.repo.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newRequests(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := s.List(context.Background(), tt.caller, tt.status, tt.size, tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (len(got.Requests) != 1 || got.TotalCount != 1 || got.NextPageToken != "") {
				t.Errorf("got %+v", got)
			}
		})
	}
}

func TestRequests_Approve(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		id      string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "admin approves pending request", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "employer is denied", caller: employer, id: requestID, wantErr: domain.ErrAdminRoleRequired},
		{name: "applicant is denied", caller: applicant, id: requestID, wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the ID", caller: employer, id: "bad", wantErr: domain.ErrAdminRoleRequired},
		{name: "request ID not a UUID", caller: admin, id: "req-1", wantErr: domain.ErrInvalidRequestID},
		{
			name: "unknown request", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(domain.VerificationRequest{}, domain.ErrRequestNotFound)
			},
			wantErr: domain.ErrRequestNotFound,
		},
		{
			name: "already approved", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusApproved), nil)
				d.clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrRequestNotPending,
		},
		{
			name: "already rejected", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusRejected), nil)
				d.clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrRequestNotPending,
		},
		{
			name: "another admin decided first", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(domain.VerificationRequest{}, domain.ErrRequestNotPending)
			},
			wantErr: domain.ErrRequestNotPending,
		},
		{
			name: "lookup error is passed through", caller: admin, id: requestID,
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(domain.VerificationRequest{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newRequests(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := s.Approve(context.Background(), tt.caller, tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			want := stored(domain.StatusApproved)
			want.ReviewedBy, want.UpdatedAt = adminID, now
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestRequests_Reject(t *testing.T) {
	for _, tt := range []struct {
		name    string
		caller  domain.Caller
		id      string
		reason  string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "admin rejects pending request with trimmed reason", caller: admin, id: requestID, reason: " Expired licence ",
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any()).DoAndReturn(returnArg)
			},
		},
		{name: "employer is denied", caller: employer, id: requestID, reason: "x", wantErr: domain.ErrAdminRoleRequired},
		{name: "applicant is denied", caller: applicant, id: requestID, reason: "x", wantErr: domain.ErrAdminRoleRequired},
		{name: "role is checked before the reason", caller: employer, id: requestID, reason: "", wantErr: domain.ErrAdminRoleRequired},
		{name: "request ID not a UUID", caller: admin, id: "", reason: "x", wantErr: domain.ErrInvalidRequestID},
		{name: "empty reason", caller: admin, id: requestID, reason: "", wantErr: domain.ErrInvalidRejectionReason},
		{name: "spaces-only reason", caller: admin, id: requestID, reason: "   ", wantErr: domain.ErrInvalidRejectionReason},
		{
			name: "unknown request", caller: admin, id: requestID, reason: "x",
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(domain.VerificationRequest{}, domain.ErrRequestNotFound)
			},
			wantErr: domain.ErrRequestNotFound,
		},
		{
			name: "already approved", caller: admin, id: requestID, reason: "x",
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusApproved), nil)
				d.clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrRequestNotPending,
		},
		{
			name: "already rejected", caller: admin, id: requestID, reason: "x",
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusRejected), nil)
				d.clock.EXPECT().Now().Return(now)
			},
			wantErr: domain.ErrRequestNotPending,
		},
		{
			name: "decide error is passed through", caller: admin, id: requestID, reason: "x",
			setup: func(d deps) {
				d.repo.EXPECT().GetByID(gomock.Any(), requestID).Return(stored(domain.StatusPending), nil)
				d.clock.EXPECT().Now().Return(now)
				d.repo.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(domain.VerificationRequest{}, errDB)
			},
			wantErr: errDB,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newRequests(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := s.Reject(context.Background(), tt.caller, tt.id, tt.reason)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			want := stored(domain.StatusRejected)
			want.ReviewedBy, want.RejectionReason, want.UpdatedAt = adminID, "Expired licence", now
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}
