//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
)

// Microsecond precision, because Postgres stores microseconds.
var base = time.Date(2026, 11, 8, 4, 0, 0, 123456000, time.UTC)

// startAuditLogDB follows the JT-20 pattern used by moderation: a real
// Postgres with all migrations applied, run twice to prove they are safe on
// every startup.
func startAuditLogDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("audit_log_test"), pgcontainer.WithUsername("audit_log_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool
}

// eventID builds an ID like "e0000000-0000-4000-8000-000000000007".
func eventID(n int) string {
	return fmt.Sprintf("e0000000-0000-4000-8000-%012d", n)
}

func newEntry(t *testing.T, n int, eventType string, at time.Time) domain.Entry {
	t.Helper()
	e, err := domain.NewEntry(eventID(n), eventType, "job-posting", at, fmt.Sprintf(`{"n":%d}`, n))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func mustRecord(t *testing.T, ctx context.Context, repo *postgres.Entries, e domain.Entry) {
	t.Helper()
	recorded, err := repo.RecordEntry(ctx, e)
	if err != nil || !recorded {
		t.Fatalf("recorded=%v err=%v, want true and nil", recorded, err)
	}
}

func count(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, id string) int {
	t.Helper()
	var n int
	// table is a fixed name from these tests, never user input.
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE event_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func listAll(t *testing.T, ctx context.Context, repo *postgres.Entries) []domain.Entry {
	t.Helper()
	items, _, err := repo.ListEntries(ctx, domain.ListFilter{}, domain.Page{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestEntries_MigrationsRecorded(t *testing.T) {
	ctx, pool := startAuditLogDB(t)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 2 AND is_applied`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("migration 2 recorded %d times, err %v, want exactly once", n, err)
	}
}

func TestEntries_RecordEntry(t *testing.T) {
	ctx, pool := startAuditLogDB(t)
	repo := postgres.NewEntries(pool)
	want := newEntry(t, 1, "job.published", base)

	mustRecord(t, ctx, repo, want)
	got := listAll(t, ctx, repo)
	if len(got) != 1 || got[0] != want || got[0].OccurredAt.Location() != time.UTC {
		t.Fatalf("stored %+v, want [%+v] in UTC", got, want)
	}

	t.Run("event ID is marked processed", func(t *testing.T) {
		if n := count(t, ctx, pool, "processed_events", want.EventID); n != 1 {
			t.Fatalf("processed rows=%d, want 1", n)
		}
	})
	t.Run("duplicate event ID is skipped without error", func(t *testing.T) {
		dup := want
		dup.DataJSON = `{"changed":true}`
		recorded, err := repo.RecordEntry(ctx, dup)
		if err != nil || recorded {
			t.Fatalf("recorded=%v err=%v, want false and nil", recorded, err)
		}
		if got := listAll(t, ctx, repo); len(got) != 1 || got[0] != want {
			t.Fatalf("after duplicate %+v, want the original only", got)
		}
	})
	t.Run("recorded_at is set by the database", func(t *testing.T) {
		var recordedAt time.Time
		if err := pool.QueryRow(ctx, `SELECT recorded_at FROM audit_entries WHERE event_id = $1`, want.EventID).Scan(&recordedAt); err != nil || recordedAt.IsZero() {
			t.Fatalf("recorded_at=%v err=%v", recordedAt, err)
		}
	})
	t.Run("data is returned exactly as received", func(t *testing.T) {
		// JSONB would reorder these keys and drop the spaces.
		raw := `{ "z": 1,   "a": "\u0000é" }`
		e, err := domain.NewEntry(eventID(2), "offer.made", "offer", base, raw)
		if err != nil {
			t.Fatal(err)
		}
		mustRecord(t, ctx, repo, e)
		items, _, err := repo.ListEntries(ctx, domain.ListFilter{EventType: "offer.made"}, domain.Page{Limit: 20})
		if err != nil || len(items) != 1 || items[0].DataJSON != raw {
			t.Fatalf("got %+v err %v, want data %q", items, err, raw)
		}
	})
}

// Two deliveries of the same event at once: exactly one is recorded and
// neither is an error.
func TestEntries_RecordEntry_ConcurrentDuplicates(t *testing.T) {
	ctx, pool := startAuditLogDB(t)
	repo := postgres.NewEntries(pool)
	e := newEntry(t, 1, "application.submitted", base)

	for round := range 10 {
		e.EventID = eventID(100 + round)
		var wg sync.WaitGroup
		results := make([]bool, 2)
		errs := make([]error, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = repo.RecordEntry(ctx, e)
			}()
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("round %d: errors %v", round, errs)
		}
		if results[0] == results[1] {
			t.Fatalf("round %d: recorded=%v, want exactly one true", round, results)
		}
		if n := count(t, ctx, pool, "audit_entries", e.EventID); n != 1 {
			t.Fatalf("round %d: %d entries, want 1", round, n)
		}
	}
}

func TestEntries_ListEntries(t *testing.T) {
	ctx, pool := startAuditLogDB(t)
	repo := postgres.NewEntries(pool)
	// Entry 1 at base, 2 at +1h, 3 at +2h, 4 and 5 both at +3h (same time, so
	// event_id breaks the tie: 5 before 4). 2 and 4 are offer.made.
	types := []string{"job.published", "offer.made", "job.published", "offer.made", "job.published"}
	offsets := []time.Duration{0, time.Hour, 2 * time.Hour, 3 * time.Hour, 3 * time.Hour}
	for i := range 5 {
		mustRecord(t, ctx, repo, newEntry(t, i+1, types[i], base.Add(offsets[i])))
	}
	ids := func(items []domain.Entry) string {
		var b strings.Builder
		for _, v := range items {
			b.WriteString(v.EventID[len(v.EventID)-1:])
		}
		return b.String()
	}
	at := func(d time.Duration) *time.Time {
		v := base.Add(d)
		return &v
	}

	for _, tt := range []struct {
		name      string
		filter    domain.ListFilter
		page      domain.Page
		wantIDs   string
		wantTotal int64
	}{
		{"all newest first with ID tie-break", domain.ListFilter{}, domain.Page{Limit: 20}, "54321", 5},
		{"first page", domain.ListFilter{}, domain.Page{Limit: 2}, "54", 5},
		{"second page", domain.ListFilter{}, domain.Page{Limit: 2, Offset: 2}, "32", 5},
		{"last page", domain.ListFilter{}, domain.Page{Limit: 2, Offset: 4}, "1", 5},
		{"offset past the end keeps the total", domain.ListFilter{}, domain.Page{Limit: 2, Offset: 10}, "", 5},
		{"event type filter", domain.ListFilter{EventType: "offer.made"}, domain.Page{Limit: 20}, "42", 2},
		{"unused event type", domain.ListFilter{EventType: "user.registered"}, domain.Page{Limit: 20}, "", 0},
		{"start is inclusive", domain.ListFilter{Start: at(2 * time.Hour)}, domain.Page{Limit: 20}, "543", 3},
		{"end is exclusive", domain.ListFilter{End: at(2 * time.Hour)}, domain.Page{Limit: 20}, "21", 2},
		{"start and end", domain.ListFilter{Start: at(time.Hour), End: at(3 * time.Hour)}, domain.Page{Limit: 20}, "32", 2},
		{"one microsecond window", domain.ListFilter{Start: at(time.Hour), End: at(time.Hour + time.Microsecond)}, domain.Page{Limit: 20}, "2", 1},
		{"all filters with paging", domain.ListFilter{EventType: "job.published", Start: at(0), End: at(4 * time.Hour)}, domain.Page{Limit: 2, Offset: 1}, "31", 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := repo.ListEntries(ctx, tt.filter, tt.page)
			if err != nil || ids(items) != tt.wantIDs || total != tt.wantTotal {
				t.Fatalf("got ids %q total %d err %v, want %q %d", ids(items), total, err, tt.wantIDs, tt.wantTotal)
			}
		})
	}
}

// The database limits back up the domain limits. These entries skip
// domain.NewEntry on purpose. A refused entry must not be marked processed,
// otherwise a redelivery would be skipped and the event lost.
func TestEntries_DatabaseLimits(t *testing.T) {
	ctx, pool := startAuditLogDB(t)
	repo := postgres.NewEntries(pool)
	jsonOfBytes := func(n int) string { return `{"k":"` + strings.Repeat("x", n-8) + `"}` }

	for i, tt := range []struct {
		name    string
		change  func(*domain.Entry)
		wantErr bool
	}{
		{"100-character event type", func(e *domain.Entry) { e.EventType = "a." + strings.Repeat("b", 98) }, false},
		{"101-character event type", func(e *domain.Entry) { e.EventType = "a." + strings.Repeat("b", 99) }, true},
		{"empty event type", func(e *domain.Entry) { e.EventType = "" }, true},
		{"50-character producer", func(e *domain.Entry) { e.Producer = strings.Repeat("a", 50) }, false},
		{"51-character producer", func(e *domain.Entry) { e.Producer = strings.Repeat("a", 51) }, true},
		{"empty producer", func(e *domain.Entry) { e.Producer = "" }, true},
		{"65536-byte data", func(e *domain.Entry) { e.DataJSON = jsonOfBytes(65536) }, false},
		{"65537-byte data", func(e *domain.Entry) { e.DataJSON = jsonOfBytes(65537) }, true},
		{"65538 bytes of 2-byte characters", func(e *domain.Entry) { e.DataJSON = `{"k":"` + strings.Repeat("é", 32765) + `"}` }, true},
		{"one-byte data", func(e *domain.Entry) { e.DataJSON = "x" }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := domain.Entry{
				EventID:    eventID(200 + i),
				EventType:  "job.published",
				Producer:   "job-posting",
				OccurredAt: base,
				DataJSON:   `{}`,
			}
			tt.change(&e)
			recorded, err := repo.RecordEntry(ctx, e)
			if tt.wantErr != (err != nil) || recorded == tt.wantErr {
				t.Fatalf("recorded=%v err=%v, wantErr=%v", recorded, err, tt.wantErr)
			}
			wantProcessed := 1
			if tt.wantErr {
				wantProcessed = 0
			}
			if n := count(t, ctx, pool, "processed_events", e.EventID); n != wantProcessed {
				t.Fatalf("processed rows=%d, want %d", n, wantProcessed)
			}
		})
	}

	t.Run("event ID that is not a UUID", func(t *testing.T) {
		e := domain.Entry{EventID: "event-1", EventType: "job.published", Producer: "job-posting", OccurredAt: base, DataJSON: `{}`}
		if _, err := repo.RecordEntry(ctx, e); err == nil {
			t.Fatal("expected an error for a non-UUID event ID")
		}
	})
	t.Run("a refused event can be recorded on redelivery", func(t *testing.T) {
		e := domain.Entry{EventID: eventID(300), EventType: "job.published", Producer: strings.Repeat("a", 51), OccurredAt: base, DataJSON: `{}`}
		if _, err := repo.RecordEntry(ctx, e); err == nil {
			t.Fatal("expected the first attempt to fail")
		}
		e.Producer = "job-posting"
		mustRecord(t, ctx, repo, e)
	})
}
