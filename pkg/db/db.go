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
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return errors.New("migration directory is not accessible")
	}
	// The bridge returns acquired connections without closing the caller's pool.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS(directory),
		goose.WithDisableGlobalRegistry(true), goose.WithLogger(goose.NopLogger()))
	if err != nil {
		return errors.New("load SQL migrations failed")
	}
	if _, err := provider.Up(ctx); err != nil {
		return safeError(ctx, "apply SQL migrations")
	}
	return nil
}

// Driver and migration errors can contain credentials or SQL literals.
func safeError(ctx context.Context, operation string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s failed", operation)
}
