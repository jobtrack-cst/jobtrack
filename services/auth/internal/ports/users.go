package ports

//go:generate go tool mockgen -destination=mocks/users_mock.go -package=mocks . UserRepository,PasswordHasher,TokenIssuer,Clock

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
)

// UserRepository stores accounts. The database generates user IDs.
type UserRepository interface {
	// Create returns domain.ErrEmailTaken if the email is already registered.
	Create(ctx context.Context, user domain.User) (domain.User, error)
	// GetByEmail and GetByID return domain.ErrUserNotFound when there is no match.
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	// ListByRole returns one page ordered by email, plus the total for that role.
	ListByRole(ctx context.Context, role domain.Role, limit, offset int) ([]domain.User, int64, error)
	// ExistsWithRole lets the admin seed skip when any admin already exists.
	ExistsWithRole(ctx context.Context, role domain.Role) (bool, error)
}

// PasswordHasher hides bcrypt from the app layer so tests stay fast.
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns domain.ErrInvalidCredentials when the password does not match.
	// An empty hash still does a full (dummy) check and returns ErrInvalidCredentials,
	// so login takes the same time whether or not the email exists.
	Compare(hash, password string) error
}

// TokenIssuer signs access tokens with the claims in AGENTS.md Section 4.
type TokenIssuer interface {
	Issue(userID string, role domain.Role, issuedAt time.Time) (token string, expiresAt time.Time, err error)
}

// Clock makes issued-at and expiry times testable.
type Clock interface {
	Now() time.Time
}
