package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// OpenPool connects and verifies a PostgreSQL pool. The caller must close it.
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	// Force UTC to prevent timezone mismatches in tests and production.
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, safeError(ctx, "initialize database pool")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, safeError(ctx, "connect to database")
	}
	return pool, nil
}

// Migrate applies pending goose SQL migrations. The caller retains ownership of pool.
func Migrate(ctx context.Context, pool *pgxpool.Pool, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if pool == nil {
		return errors.New("database pool is required")
	}
	if strings.TrimSpace(directory) == "" {
		return errors.New("migration directory is required")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return errors.New("unsupported database dialect")
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	if err := goose.UpContext(ctx, db, directory); err != nil {
		return safeError(ctx, "apply database migrations")
	}
	return nil
}

func safeError(ctx context.Context, operation string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%s failed", operation)
}

var _ = os.Getenv // keep os import if needed
