package app

import (
	"context"
	"errors"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports"
)

// AdminFullName is used for the seeded admin; ADMIN_EMAIL and ADMIN_PASSWORD carry no name.
const AdminFullName = "Placement Coordinator"

// Auth holds the auth use cases. Handlers translate its domain errors to gRPC codes.
type Auth struct {
	users  ports.UserRepository
	hasher ports.PasswordHasher
	tokens ports.TokenIssuer
	clock  ports.Clock
}

func NewAuth(users ports.UserRepository, hasher ports.PasswordHasher, tokens ports.TokenIssuer, clock ports.Clock) *Auth {
	return &Auth{users: users, hasher: hasher, tokens: tokens, clock: clock}
}

// LoginResult is what a successful login returns.
type LoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	User        domain.User
}

// UserPage is one page of ListUsersByRole.
type UserPage struct {
	Users         []domain.User
	NextPageToken string
	TotalCount    int64
}

// Register creates an applicant or employer. Admins are rejected by the domain rule.
func (a *Auth) Register(ctx context.Context, email, password, fullName string, role domain.Role) (domain.User, error) {
	reg, err := domain.NewRegistration(email, password, fullName, role)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := a.hasher.Hash(reg.Password)
	if err != nil {
		return domain.User{}, err
	}
	return a.users.Create(ctx, domain.User{
		Email:        reg.Email,
		FullName:     reg.FullName,
		Role:         reg.Role,
		PasswordHash: hash,
	})
}

// Login returns ErrInvalidCredentials for every kind of wrong email or password.
func (a *Auth) Login(ctx context.Context, email, password string) (LoginResult, error) {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return LoginResult{}, domain.ErrInvalidCredentials
	}
	user, err := a.users.GetByEmail(ctx, normalized)
	if errors.Is(err, domain.ErrUserNotFound) {
		return LoginResult{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if err := a.hasher.Compare(user.PasswordHash, password); err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return LoginResult{}, domain.ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	token, expiresAt, err := a.tokens.Issue(user.ID, user.Role, a.clock.Now())
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccessToken: token, ExpiresAt: expiresAt, User: user}, nil
}

// GetCurrentUser returns the caller's own account. Any role may call it.
func (a *Auth) GetCurrentUser(ctx context.Context, callerID string) (domain.User, error) {
	if callerID == "" {
		return domain.User{}, domain.ErrUnauthenticated
	}
	if err := domain.ValidateUserID(callerID); err != nil {
		return domain.User{}, domain.ErrUnauthenticated
	}
	return a.users.GetByID(ctx, callerID)
}

// GetUser is internal (notification-dispatcher), so it has no role check.
func (a *Auth) GetUser(ctx context.Context, userID string) (domain.User, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return domain.User{}, err
	}
	return a.users.GetByID(ctx, userID)
}

// ListUsersByRole is internal (weekly-summary), so it has no role check.
func (a *Auth) ListUsersByRole(ctx context.Context, role domain.Role, pageSize int32, pageToken string) (UserPage, error) {
	if !role.Valid() {
		return UserPage{}, domain.ErrInvalidRole
	}
	page, err := domain.NewPage(pageSize, pageToken)
	if err != nil {
		return UserPage{}, err
	}
	users, total, err := a.users.ListByRole(ctx, role, page.Limit, page.Offset)
	if err != nil {
		return UserPage{}, err
	}
	return UserPage{Users: users, NextPageToken: page.NextToken(len(users), total), TotalCount: total}, nil
}

// SeedAdmin creates one admin if none exists and never updates an existing admin.
// The password is checked first, so a weak ADMIN_PASSWORD always stops startup.
func (a *Auth) SeedAdmin(ctx context.Context, email, password string) (created bool, err error) {
	if err := domain.ValidateAdminPassword(password); err != nil {
		return false, err
	}
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	exists, err := a.users.ExistsWithRole(ctx, domain.RoleAdmin)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	hash, err := a.hasher.Hash(password)
	if err != nil {
		return false, err
	}
	if _, err := a.users.Create(ctx, domain.User{
		Email:        normalized,
		FullName:     AdminFullName,
		Role:         domain.RoleAdmin,
		PasswordHash: hash,
	}); err != nil {
		return false, err
	}
	return true, nil
}
