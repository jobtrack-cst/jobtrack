package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	adapter "github.com/jobtrack-cst/jobtrack/services/auth/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testUserID = "3f2b8c1e-1111-4111-8111-111111111111"
	testEmail  = "pema@cst.edu.bt"
	testPass   = "password123"
	testHash   = "$2a$10$hash"
)

var (
	errDB    = errors.New("pq: secret SQL detail")
	testTime = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	stored   = domain.User{ID: testUserID, Email: testEmail, FullName: "Pema", Role: domain.RoleEmployer,
		PasswordHash: testHash, CreatedAt: testTime, UpdatedAt: testTime}
)

type deps struct {
	users  *mocks.MockUserRepository
	hasher *mocks.MockPasswordHasher
	tokens *mocks.MockTokenIssuer
	clock  *mocks.MockClock
}

// Handlers are tested with the real app layer and mocked ports.
func newHandlers(t *testing.T) (*adapter.AuthHandler, *adapter.InternalHandler, deps) {
	ctrl := gomock.NewController(t)
	d := deps{
		users: mocks.NewMockUserRepository(ctrl), hasher: mocks.NewMockPasswordHasher(ctrl),
		tokens: mocks.NewMockTokenIssuer(ctrl), clock: mocks.NewMockClock(ctrl),
	}
	auth := app.NewAuth(d.users, d.hasher, d.tokens, d.clock)
	return adapter.NewAuthHandler(auth), adapter.NewInternalHandler(auth), d
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

func TestAuthHandler_Register(t *testing.T) {
	for _, tt := range []struct {
		name  string
		req   *authv1.RegisterRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "employer registers", want: codes.OK,
			req: &authv1.RegisterRequest{Email: testEmail, Password: testPass, FullName: "Pema", Role: authv1.Role_ROLE_EMPLOYER},
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPass).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(stored, nil)
			},
		},
		{name: "admin role is rejected", want: codes.InvalidArgument,
			req: &authv1.RegisterRequest{Email: testEmail, Password: testPass, FullName: "Pema", Role: authv1.Role_ROLE_ADMIN}},
		{name: "unspecified role is rejected", want: codes.InvalidArgument,
			req: &authv1.RegisterRequest{Email: testEmail, Password: testPass, FullName: "Pema"}},
		{name: "unknown enum value is rejected", want: codes.InvalidArgument,
			req: &authv1.RegisterRequest{Email: testEmail, Password: testPass, FullName: "Pema", Role: authv1.Role(99)}},
		{name: "bad email", want: codes.InvalidArgument,
			req: &authv1.RegisterRequest{Email: "nope", Password: testPass, FullName: "Pema", Role: authv1.Role_ROLE_APPLICANT}},
		{name: "nil request", want: codes.InvalidArgument},
		{
			name: "duplicate email", want: codes.AlreadyExists,
			req: &authv1.RegisterRequest{Email: testEmail, Password: testPass, FullName: "Pema", Role: authv1.Role_ROLE_APPLICANT},
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPass).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, d := newHandlers(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.Register(context.Background(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				u := resp.GetUser()
				if u.GetUserId() != testUserID || u.GetRole() != authv1.Role_ROLE_EMPLOYER || !u.GetTimestamps().GetCreatedAt().AsTime().Equal(testTime) {
					t.Fatalf("unexpected user %v", u)
				}
			}
		})
	}
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("success returns token, expiry and user", func(t *testing.T) {
		h, _, d := newHandlers(t)
		d.users.EXPECT().GetByEmail(gomock.Any(), testEmail).Return(stored, nil)
		d.hasher.EXPECT().Compare(testHash, testPass).Return(nil)
		d.clock.EXPECT().Now().Return(testTime)
		d.tokens.EXPECT().Issue(testUserID, domain.RoleEmployer, testTime).Return("jwt", testTime.Add(time.Hour), nil)

		resp, err := h.Login(context.Background(), &authv1.LoginRequest{Email: testEmail, Password: testPass})
		wantCode(t, err, codes.OK)
		if resp.GetAccessToken() != "jwt" || !resp.GetExpiresAt().AsTime().Equal(testTime.Add(time.Hour)) || resp.GetUser().GetEmail() != testEmail {
			t.Fatalf("unexpected response %v", resp)
		}
	})

	t.Run("wrong email and wrong password give identical errors", func(t *testing.T) {
		h, _, d := newHandlers(t)
		d.users.EXPECT().GetByEmail(gomock.Any(), "nobody@cst.edu.bt").Return(domain.User{}, domain.ErrUserNotFound)
		d.hasher.EXPECT().Compare("", testPass).Return(domain.ErrInvalidCredentials)
		d.users.EXPECT().GetByEmail(gomock.Any(), testEmail).Return(stored, nil)
		d.hasher.EXPECT().Compare(testHash, testPass).Return(domain.ErrInvalidCredentials)

		_, wrongEmail := h.Login(context.Background(), &authv1.LoginRequest{Email: "nobody@cst.edu.bt", Password: testPass})
		_, wrongPass := h.Login(context.Background(), &authv1.LoginRequest{Email: testEmail, Password: testPass})
		wantCode(t, wrongEmail, codes.Unauthenticated)
		if status.Convert(wrongEmail).Message() != status.Convert(wrongPass).Message() || status.Code(wrongPass) != codes.Unauthenticated {
			t.Fatalf("errors differ: %v / %v", wrongEmail, wrongPass)
		}
	})

	t.Run("database failure is INTERNAL without leaking details", func(t *testing.T) {
		h, _, d := newHandlers(t)
		d.users.EXPECT().GetByEmail(gomock.Any(), testEmail).Return(domain.User{}, errDB)
		_, err := h.Login(context.Background(), &authv1.LoginRequest{Email: testEmail, Password: testPass})
		wantCode(t, err, codes.Internal)
		if status.Convert(err).Message() != "internal error" {
			t.Fatalf("message leaked: %q", status.Convert(err).Message())
		}
	})
}

func TestAuthHandler_GetCurrentUser(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "any valid role gets its own account", ctx: withCaller(testUserID, "applicant"), want: codes.OK,
			setup: func(d deps) { d.users.EXPECT().GetByID(gomock.Any(), testUserID).Return(stored, nil) },
		},
		{name: "no metadata", ctx: context.Background(), want: codes.Unauthenticated},
		{name: "unknown role", ctx: withCaller(testUserID, "superuser"), want: codes.Unauthenticated},
		{name: "malformed user ID", ctx: withCaller("abc", "admin"), want: codes.Unauthenticated},
		{
			name: "account deleted", ctx: withCaller(testUserID, "employer"), want: codes.NotFound,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), testUserID).Return(domain.User{}, domain.ErrUserNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, d := newHandlers(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.GetCurrentUser(tt.ctx, &authv1.GetCurrentUserRequest{})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && resp.GetUser().GetUserId() != testUserID {
				t.Fatalf("unexpected user %v", resp.GetUser())
			}
		})
	}
}

func TestInternalHandler_GetUser(t *testing.T) {
	for _, tt := range []struct {
		name  string
		id    string
		setup func(d deps)
		want  codes.Code
	}{
		{name: "found", id: testUserID, want: codes.OK,
			setup: func(d deps) { d.users.EXPECT().GetByID(gomock.Any(), testUserID).Return(stored, nil) }},
		{name: "malformed ID", id: "abc", want: codes.InvalidArgument},
		{name: "not found", id: testUserID, want: codes.NotFound,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), testUserID).Return(domain.User{}, domain.ErrUserNotFound)
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, h, d := newHandlers(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.GetUser(context.Background(), &authv1.GetUserRequest{UserId: tt.id})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && resp.GetUser().GetEmail() != testEmail {
				t.Fatalf("unexpected user %v", resp.GetUser())
			}
		})
	}
}

func TestInternalHandler_ListUsersByRole(t *testing.T) {
	admin := domain.User{ID: testUserID, Email: "coordinator@cst.edu.bt", Role: domain.RoleAdmin}
	for _, tt := range []struct {
		name     string
		req      *authv1.ListUsersByRoleRequest
		setup    func(d deps)
		want     codes.Code
		wantNext string
	}{
		{
			name: "first page of admins", want: codes.OK, wantNext: "1",
			req: &authv1.ListUsersByRoleRequest{Role: authv1.Role_ROLE_ADMIN, Pagination: &commonv1.PaginationRequest{PageSize: 1}},
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleAdmin, 1, 0).Return([]domain.User{admin}, int64(2), nil)
			},
		},
		{
			name: "missing pagination uses the default page size", want: codes.OK,
			req: &authv1.ListUsersByRoleRequest{Role: authv1.Role_ROLE_ADMIN},
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleAdmin, 20, 0).Return([]domain.User{admin}, int64(1), nil)
			},
		},
		{name: "unspecified role", want: codes.InvalidArgument, req: &authv1.ListUsersByRoleRequest{}},
		{name: "bad page token", want: codes.InvalidArgument,
			req: &authv1.ListUsersByRoleRequest{Role: authv1.Role_ROLE_ADMIN, Pagination: &commonv1.PaginationRequest{PageToken: "x"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, h, d := newHandlers(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.ListUsersByRole(context.Background(), tt.req)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				if resp.GetPagination().GetNextPageToken() != tt.wantNext || len(resp.GetUsers()) != 1 ||
					resp.GetUsers()[0].GetRole() != authv1.Role_ROLE_ADMIN {
					t.Fatalf("unexpected response %v", resp)
				}
			}
		})
	}
}
