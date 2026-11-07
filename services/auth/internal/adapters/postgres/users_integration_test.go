//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// startAuthDB follows the template pattern: a real Postgres with all migrations applied.
func startAuthDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("auth_test"), pgcontainer.WithUsername("auth_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get test connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Running twice proves migrations are safe to repeat on every startup.
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool
}

func newUser(email string, role domain.Role) domain.User {
	return domain.User{Email: email, FullName: "Test User", Role: role, PasswordHash: "$2a$10$synthetic"}
}

func TestUsers_CreateAndGet(t *testing.T) {
	ctx, pool := startAuthDB(t)
	repo := postgres.NewUsers(pool)

	created, err := repo.Create(ctx, newUser("pema@cst.edu.bt", domain.RoleEmployer))
	if err != nil {
		t.Fatal(err)
	}
	if domain.ValidateUserID(created.ID) != nil || created.CreatedAt.IsZero() || created.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected generated UUID and UTC timestamps, got %+v", created)
	}

	for name, get := range map[string]func() (domain.User, error){
		"by email": func() (domain.User, error) { return repo.GetByEmail(ctx, "pema@cst.edu.bt") },
		"by ID":    func() (domain.User, error) { return repo.GetByID(ctx, created.ID) },
	} {
		t.Run(name, func(t *testing.T) {
			got, err := get()
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != created.ID || got.Role != domain.RoleEmployer || got.PasswordHash != "$2a$10$synthetic" {
				t.Fatalf("got %+v, want %+v", got, created)
			}
		})
	}

	t.Run("duplicate email", func(t *testing.T) {
		if _, err := repo.Create(ctx, newUser("pema@cst.edu.bt", domain.RoleApplicant)); !errors.Is(err, domain.ErrEmailTaken) {
			t.Fatalf("err=%v, want ErrEmailTaken", err)
		}
	})
	t.Run("unknown email", func(t *testing.T) {
		if _, err := repo.GetByEmail(ctx, "nobody@cst.edu.bt"); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err=%v, want ErrUserNotFound", err)
		}
	})
	t.Run("unknown ID", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err=%v, want ErrUserNotFound", err)
		}
	})
	t.Run("database rejects uppercase email", func(t *testing.T) {
		if _, err := repo.Create(ctx, newUser("Upper@cst.edu.bt", domain.RoleApplicant)); err == nil {
			t.Fatal("expected the lowercase CHECK constraint to reject the insert")
		}
	})
}

func TestUsers_ListByRoleAndExistsWithRole(t *testing.T) {
	ctx, pool := startAuthDB(t)
	repo := postgres.NewUsers(pool)

	exists, err := repo.ExistsWithRole(ctx, domain.RoleAdmin)
	if err != nil || exists {
		t.Fatalf("empty database: exists=%v err=%v", exists, err)
	}
	// Inserted out of order to prove the list is sorted by email.
	for _, email := range []string{"c@cst.edu.bt", "a@cst.edu.bt", "b@cst.edu.bt"} {
		if _, err := repo.Create(ctx, newUser(email, domain.RoleAdmin)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Create(ctx, newUser("applicant@cst.edu.bt", domain.RoleApplicant)); err != nil {
		t.Fatal(err)
	}

	exists, err = repo.ExistsWithRole(ctx, domain.RoleAdmin)
	if err != nil || !exists {
		t.Fatalf("after seeding: exists=%v err=%v", exists, err)
	}

	for _, tt := range []struct {
		limit, offset int
		want          []string
	}{
		{2, 0, []string{"a@cst.edu.bt", "b@cst.edu.bt"}},
		{2, 2, []string{"c@cst.edu.bt"}},
		{2, 4, []string{}},
	} {
		t.Run(fmt.Sprintf("limit %d offset %d", tt.limit, tt.offset), func(t *testing.T) {
			users, total, err := repo.ListByRole(ctx, domain.RoleAdmin, tt.limit, tt.offset)
			if err != nil {
				t.Fatal(err)
			}
			if total != 3 || len(users) != len(tt.want) {
				t.Fatalf("total=%d len=%d, want 3 and %d", total, len(users), len(tt.want))
			}
			for i, u := range users {
				if u.Email != tt.want[i] || u.Role != domain.RoleAdmin {
					t.Fatalf("users[%d]=%s/%s, want %s/admin", i, u.Email, u.Role, tt.want[i])
				}
			}
		})
	}
}
