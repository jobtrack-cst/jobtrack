package db_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
)

func TestOpenPool_RejectsInvalidConfiguration(t *testing.T) {
	for _, url := range []string{"", "  ", "postgres://user:secret@localhost:invalid/db"} {
		pool, err := db.OpenPool(context.Background(), url)
		if err == nil || pool != nil {
			t.Fatalf("expected error and nil pool")
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), url) && strings.TrimSpace(url) != "" {
			t.Fatal("error exposes database configuration")
		}
	}
}

func TestOpenPool_RejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool, err := db.OpenPool(ctx, "postgres://user:secret@localhost/db")
	if pool != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context and nil pool, got %v", err)
	}
}

func TestOpenPool_ReturnsConnectionFailureWithoutCredentials(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := db.OpenPool(ctx, "postgres://test_user:secret@"+address+"/test_db?sslmode=disable")
	if err == nil || pool != nil {
		t.Fatal("expected connection failure and nil pool")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "test_user") {
		t.Fatal("error exposes credentials")
	}
}

func TestMigrate_RejectsInvalidInputs(t *testing.T) {
	// Creating a lazy pool needs no running PostgreSQL instance.
	pool, err := pgxpool.New(context.Background(), "postgres://test_user:secret@127.0.0.1:1/test_db?sslmode=disable")
	if err != nil {
		t.Fatal("create lazy pool failed")
	}
	defer pool.Close()
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		pool *pgxpool.Pool
		dir  string
	}{
		{"nil pool", nil, t.TempDir()},
		{"empty directory", pool, ""},
		{"missing directory", pool, filepath.Join(t.TempDir(), "missing")},
		{"file instead of directory", pool, file},
		{"no migrations", pool, t.TempDir()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.Migrate(context.Background(), tt.pool, tt.dir); err == nil {
				t.Fatal("expected validation or migration loading error")
			}
			if pool.Stat().AcquiredConns() != 0 {
				t.Fatal("migration loading leaked an acquired connection")
			}
		})
	}
}

func TestMigrate_PreservesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := db.Migrate(ctx, nil, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
