package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/auth/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

const (
	userID   = "3f2b8c1e-1111-4111-8111-111111111111"
	email    = "pema@cst.edu.bt"
	password = "password123"
	hash     = "$2a$10$hash"
)

var (
	errDB = errors.New("database down")
	now   = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
)

type deps struct {
	users  *mocks.MockUserRepository
	hasher *mocks.MockPasswordHasher
	tokens *mocks.MockTokenIssuer
	clock  *mocks.MockClock
}

func newAuth(t *testing.T) (*app.Auth, deps) {
	ctrl := gomock.NewController(t)
	d := deps{
		users:  mocks.NewMockUserRepository(ctrl),
		hasher: mocks.NewMockPasswordHasher(ctrl),
		tokens: mocks.NewMockTokenIssuer(ctrl),
		clock:  mocks.NewMockClock(ctrl),
	}
	return app.NewAuth(d.users, d.hasher, d.tokens, d.clock), d
}

func TestAuth_Register(t *testing.T) {
	for _, tt := range []struct {
		name    string
		role    domain.Role
		email   string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "applicant is created with a hashed password", role: domain.RoleApplicant, email: " Pema@CST.edu.bt ",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(password).Return(hash, nil)
				d.users.EXPECT().Create(gomock.Any(), domain.User{
					Email: email, FullName: "Pema", Role: domain.RoleApplicant, PasswordHash: hash,
				}).Return(domain.User{ID: userID, Email: email, Role: domain.RoleApplicant}, nil)
			},
		},
		{
			name: "employer is created", role: domain.RoleEmployer, email: email,
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(password).Return(hash, nil)
				d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{ID: userID}, nil)
			},
		},
		{name: "admin cannot self-register", role: domain.RoleAdmin, email: email, wantErr: domain.ErrRoleNotAllowed},
		{name: "unspecified role is rejected", role: domain.RoleUnspecified, email: email, wantErr: domain.ErrRoleNotAllowed},
		{name: "invalid email never reaches storage", role: domain.RoleApplicant, email: "nope", wantErr: domain.ErrInvalidEmail},
		{
			name: "duplicate email", role: domain.RoleApplicant, email: email, wantErr: domain.ErrEmailTaken,
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(password).Return(hash, nil)
				d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)
			},
		},
		{
			name: "hash failure", role: domain.RoleApplicant, email: email, wantErr: errDB,
			setup: func(d deps) { d.hasher.EXPECT().Hash(password).Return("", errDB) },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			user, err := auth.Register(context.Background(), tt.email, password, "Pema", tt.role)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && user.ID != userID {
				t.Fatalf("user ID=%q, want %q", user.ID, userID)
			}
		})
	}
}

func TestAuth_Login(t *testing.T) {
	stored := domain.User{ID: userID, Email: email, Role: domain.RoleEmployer, PasswordHash: hash}
	for _, tt := range []struct {
		name    string
		email   string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "valid credentials issue a token at the clock time", email: "PEMA@cst.edu.bt",
			setup: func(d deps) {
				d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(stored, nil)
				d.hasher.EXPECT().Compare(hash, password).Return(nil)
				d.clock.EXPECT().Now().Return(now)
				d.tokens.EXPECT().Issue(userID, domain.RoleEmployer, now).Return("jwt", now.Add(time.Hour), nil)
			},
		},
		{
			name: "unknown email still runs a dummy password check", email: email, wantErr: domain.ErrInvalidCredentials,
			setup: func(d deps) {
				d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(domain.User{}, domain.ErrUserNotFound)
				d.hasher.EXPECT().Compare("", password).Return(domain.ErrInvalidCredentials)
			},
		},
		{
			name: "wrong password", email: email, wantErr: domain.ErrInvalidCredentials,
			setup: func(d deps) {
				d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(stored, nil)
				d.hasher.EXPECT().Compare(hash, password).Return(domain.ErrInvalidCredentials)
			},
		},
		{name: "malformed email", email: "not an email", wantErr: domain.ErrInvalidCredentials},
		{
			name: "storage failure is not hidden as bad credentials", email: email, wantErr: errDB,
			setup: func(d deps) {
				d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(domain.User{}, errDB)
			},
		},
		{
			name: "token signing failure", email: email, wantErr: errDB,
			setup: func(d deps) {
				d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(stored, nil)
				d.hasher.EXPECT().Compare(hash, password).Return(nil)
				d.clock.EXPECT().Now().Return(now)
				d.tokens.EXPECT().Issue(userID, domain.RoleEmployer, now).Return("", time.Time{}, errDB)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := auth.Login(context.Background(), tt.email, password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (got.AccessToken != "jwt" || !got.ExpiresAt.Equal(now.Add(time.Hour)) || got.User.ID != userID) {
				t.Fatalf("unexpected result %+v", got)
			}
		})
	}
}

// The acceptance criteria require the same message for wrong email and wrong password.
func TestAuth_Login_SameErrorForWrongEmailAndWrongPassword(t *testing.T) {
	auth, d := newAuth(t)
	d.users.EXPECT().GetByEmail(gomock.Any(), "nobody@cst.edu.bt").Return(domain.User{}, domain.ErrUserNotFound)
	d.hasher.EXPECT().Compare("", password).Return(domain.ErrInvalidCredentials)
	d.users.EXPECT().GetByEmail(gomock.Any(), email).Return(domain.User{PasswordHash: hash}, nil)
	d.hasher.EXPECT().Compare(hash, password).Return(domain.ErrInvalidCredentials)

	_, wrongEmail := auth.Login(context.Background(), "nobody@cst.edu.bt", password)
	_, wrongPassword := auth.Login(context.Background(), email, password)
	if wrongEmail == nil || wrongPassword == nil || wrongEmail.Error() != wrongPassword.Error() {
		t.Fatalf("messages differ: %v / %v", wrongEmail, wrongPassword)
	}
}

func TestAuth_GetCurrentUser(t *testing.T) {
	for _, tt := range []struct {
		name     string
		callerID string
		setup    func(d deps)
		wantErr  error
	}{
		{
			name: "returns the caller's own account", callerID: userID,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), userID).Return(domain.User{ID: userID}, nil)
			},
		},
		{name: "missing caller", callerID: "", wantErr: domain.ErrUnauthenticated},
		{name: "malformed caller ID", callerID: "abc", wantErr: domain.ErrUnauthenticated},
		{
			name: "caller account no longer exists", callerID: userID, wantErr: domain.ErrUserNotFound,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), userID).Return(domain.User{}, domain.ErrUserNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			if _, err := auth.GetCurrentUser(context.Background(), tt.callerID); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuth_GetUser(t *testing.T) {
	for _, tt := range []struct {
		name    string
		id      string
		setup   func(d deps)
		wantErr error
	}{
		{
			name: "found", id: userID,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), userID).Return(domain.User{ID: userID}, nil)
			},
		},
		{name: "malformed ID", id: "abc", wantErr: domain.ErrInvalidUserID},
		{name: "empty ID", id: "", wantErr: domain.ErrInvalidUserID},
		{
			name: "not found", id: userID, wantErr: domain.ErrUserNotFound,
			setup: func(d deps) {
				d.users.EXPECT().GetByID(gomock.Any(), userID).Return(domain.User{}, domain.ErrUserNotFound)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			if _, err := auth.GetUser(context.Background(), tt.id); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuth_ListUsersByRole(t *testing.T) {
	admins := []domain.User{{ID: userID, Role: domain.RoleAdmin}}
	for _, tt := range []struct {
		name      string
		role      domain.Role
		size      int32
		token     string
		setup     func(d deps)
		wantNext  string
		wantTotal int64
		wantErr   error
	}{
		{
			name: "first page with more results", role: domain.RoleAdmin, size: 1, wantNext: "1", wantTotal: 3,
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleAdmin, 1, 0).Return(admins, int64(3), nil)
			},
		},
		{
			name: "last page has no next token", role: domain.RoleAdmin, size: 1, token: "2", wantTotal: 3,
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleAdmin, 1, 2).Return(admins, int64(3), nil)
			},
		},
		{
			name: "oversized page is capped at 100", role: domain.RoleApplicant, size: 500, wantTotal: 0,
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleApplicant, 100, 0).Return(nil, int64(0), nil)
			},
		},
		{name: "unspecified role", role: domain.RoleUnspecified, wantErr: domain.ErrInvalidRole},
		{name: "bad page token", role: domain.RoleAdmin, token: "x", wantErr: domain.ErrInvalidPageToken},
		{name: "negative page size", role: domain.RoleAdmin, size: -1, wantErr: domain.ErrInvalidPageSize},
		{
			name: "storage failure", role: domain.RoleAdmin, wantErr: errDB,
			setup: func(d deps) {
				d.users.EXPECT().ListByRole(gomock.Any(), domain.RoleAdmin, 20, 0).Return(nil, int64(0), errDB)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			got, err := auth.ListUsersByRole(context.Background(), tt.role, tt.size, tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (got.NextPageToken != tt.wantNext || got.TotalCount != tt.wantTotal) {
				t.Fatalf("got next=%q total=%d, want next=%q total=%d", got.NextPageToken, got.TotalCount, tt.wantNext, tt.wantTotal)
			}
		})
	}
}

func TestAuth_SeedAdmin(t *testing.T) {
	const adminEmail, strong = "coordinator@cst.edu.bt", "a-strong-admin-pass"
	for _, tt := range []struct {
		name        string
		email       string
		password    string
		setup       func(d deps)
		wantCreated bool
		wantErr     error
	}{
		{
			name: "creates the admin when none exists", email: adminEmail, password: strong, wantCreated: true,
			setup: func(d deps) {
				d.users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, nil)
				d.hasher.EXPECT().Hash(strong).Return(hash, nil)
				d.users.EXPECT().Create(gomock.Any(), domain.User{
					Email: adminEmail, FullName: app.AdminFullName, Role: domain.RoleAdmin, PasswordHash: hash,
				}).Return(domain.User{ID: userID}, nil)
			},
		},
		{
			name: "never updates an existing admin", email: adminEmail, password: strong,
			setup: func(d deps) {
				d.users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(true, nil)
			},
		},
		{name: "missing password stops startup", email: adminEmail, password: "", wantErr: domain.ErrWeakAdminPassword},
		{name: "placeholder password stops startup", email: adminEmail, password: domain.AdminPasswordPlaceholder, wantErr: domain.ErrWeakAdminPassword},
		{name: "11 character password stops startup", email: adminEmail, password: strings.Repeat("a", 11), wantErr: domain.ErrWeakAdminPassword},
		{name: "invalid admin email", email: "nope", password: strong, wantErr: domain.ErrInvalidEmail},
		{
			name: "admin email already used by another account", email: adminEmail, password: strong, wantErr: domain.ErrEmailTaken,
			setup: func(d deps) {
				d.users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, nil)
				d.hasher.EXPECT().Hash(strong).Return(hash, nil)
				d.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)
			},
		},
		{
			name: "storage failure", email: adminEmail, password: strong, wantErr: errDB,
			setup: func(d deps) {
				d.users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, errDB)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth, d := newAuth(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			created, err := auth.SeedAdmin(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) || created != tt.wantCreated {
				t.Fatalf("got %v,%v, want %v,%v", created, err, tt.wantCreated, tt.wantErr)
			}
		})
	}
}
