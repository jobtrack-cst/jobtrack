package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports"
)

const uniqueViolation = "23505"

const userColumns = "id, email, full_name, role, password_hash, created_at, updated_at"

// Users stores accounts in the auth database. The adapter never owns the pool.
type Users struct{ pool *pgxpool.Pool }

var _ ports.UserRepository = (*Users)(nil)

func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

func (r *Users) Create(ctx context.Context, user domain.User) (domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, full_name, role, password_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+userColumns,
		user.Email, user.FullName, user.Role.String(), user.PasswordHash)
	created, err := scanUser(row)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.User{}, domain.ErrEmailTaken
	}
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return domain.User{}, safe(ctx, "create user failed")
	}
	return created, nil
}

func (r *Users) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
}

func (r *Users) GetByID(ctx context.Context, id string) (domain.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

func (r *Users) getOne(ctx context.Context, query string, arg string) (domain.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, query, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, safe(ctx, "get user failed")
	}
	return user, nil
}

func (r *Users) ListByRole(ctx context.Context, role domain.Role, limit, offset int) ([]domain.User, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = $1`, role.String()).Scan(&total); err != nil {
		return nil, 0, safe(ctx, "count users failed")
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+userColumns+` FROM users WHERE role = $1 ORDER BY email LIMIT $2 OFFSET $3`,
		role.String(), limit, offset)
	if err != nil {
		return nil, 0, safe(ctx, "list users failed")
	}
	defer rows.Close()
	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, safe(ctx, "list users failed")
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, 0, safe(ctx, "list users failed")
	}
	return users, total, nil
}

func (r *Users) ExistsWithRole(ctx context.Context, role domain.Role) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role = $1)`, role.String()).Scan(&exists); err != nil {
		return false, safe(ctx, "check role failed")
	}
	return exists, nil
}

func scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User
	var role string
	if err := row.Scan(&user.ID, &user.Email, &user.FullName, &role, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return domain.User{}, err
	}
	parsed, ok := domain.ParseRole(role)
	if !ok {
		return domain.User{}, errors.New("stored role is not valid")
	}
	user.Role = parsed
	// Store and return UTC, as AGENTS.md Section 9 requires.
	user.CreatedAt, user.UpdatedAt = user.CreatedAt.UTC(), user.UpdatedAt.UTC()
	return user, nil
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
