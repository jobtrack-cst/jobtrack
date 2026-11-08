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
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

// Microsecond precision, because Postgres stores microseconds.
var base = time.Date(2026, 11, 8, 4, 0, 0, 123456000, time.UTC)

// startAnalyticsDB follows the JT-20 pattern used by audit-log: a real
// Postgres with all migrations applied, run twice to prove they are safe on
// every startup.
func startAnalyticsDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("analytics_test"), pgcontainer.WithUsername("analytics_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

func newEvent(t *testing.T, n int, eventType, status string, at time.Time) domain.OutcomeEvent {
	t.Helper()
	e, err := domain.NewOutcomeEvent(eventID(n), eventType, status, at)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func mustRecord(t *testing.T, ctx context.Context, repo *postgres.Outcomes, e domain.OutcomeEvent) {
	t.Helper()
	recorded, err := repo.RecordEvent(ctx, e)
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

func mustRange(t *testing.T, start, end time.Time) domain.TimeRange {
	t.Helper()
	r, err := domain.NewTimeRange(&start, &end)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOutcomes_MigrationsRecorded(t *testing.T) {
	ctx, pool := startAnalyticsDB(t)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 2 AND is_applied`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("migration 2 recorded %d times, err %v, want exactly once", n, err)
	}
}

func TestOutcomes_RecordEvent(t *testing.T) {
	ctx, pool := startAnalyticsDB(t)
	repo := postgres.NewOutcomes(pool)
	published := newEvent(t, 1, domain.EventJobPublished, "", base)
	changed := newEvent(t, 2, domain.EventStatusChanged, "SHORTLISTED", base)
	mustRecord(t, ctx, repo, published)
	mustRecord(t, ctx, repo, changed)

	t.Run("event IDs are marked processed", func(t *testing.T) {
		for _, id := range []string{published.EventID, changed.EventID} {
			if n := count(t, ctx, pool, "processed_events", id); n != 1 {
				t.Fatalf("%s: processed rows=%d, want 1", id, n)
			}
		}
	})
	t.Run("stored fields, with NULL status for other types and UTC times", func(t *testing.T) {
		for _, want := range []domain.OutcomeEvent{published, changed} {
			var got domain.OutcomeEvent
			var status *string
			if err := pool.QueryRow(ctx,
				`SELECT event_id::text, event_type, new_status, occurred_at FROM outcome_events WHERE event_id = $1`,
				want.EventID).Scan(&got.EventID, &got.EventType, &status, &got.OccurredAt); err != nil {
				t.Fatal(err)
			}
			if status != nil {
				got.NewStatus = *status
			}
			if (want.NewStatus == "") != (status == nil) {
				t.Fatalf("%s: status %v, want NULL only when empty", want.EventType, status)
			}
			if got.EventID != want.EventID || got.EventType != want.EventType || got.NewStatus != want.NewStatus || !got.OccurredAt.Equal(want.OccurredAt) {
				t.Fatalf("stored %+v, want %+v", got, want)
			}
		}
	})
	t.Run("duplicate event ID is skipped without error", func(t *testing.T) {
		dup := changed
		dup.NewStatus = "REJECTED"
		recorded, err := repo.RecordEvent(ctx, dup)
		if err != nil || recorded {
			t.Fatalf("recorded=%v err=%v, want false and nil", recorded, err)
		}
		var status string
		if err := pool.QueryRow(ctx, `SELECT new_status FROM outcome_events WHERE event_id = $1`, changed.EventID).Scan(&status); err != nil || status != "SHORTLISTED" {
			t.Fatalf("status=%q err=%v, want the original SHORTLISTED", status, err)
		}
		if n := count(t, ctx, pool, "outcome_events", changed.EventID); n != 1 {
			t.Fatalf("rows=%d, want 1", n)
		}
	})
	t.Run("recorded_at is set by the database", func(t *testing.T) {
		var recordedAt time.Time
		if err := pool.QueryRow(ctx, `SELECT recorded_at FROM outcome_events WHERE event_id = $1`, published.EventID).Scan(&recordedAt); err != nil || recordedAt.IsZero() {
			t.Fatalf("recorded_at=%v err=%v", recordedAt, err)
		}
	})
}

// Two deliveries of the same event at once: exactly one is recorded and
// neither is an error.
func TestOutcomes_RecordEvent_ConcurrentDuplicates(t *testing.T) {
	ctx, pool := startAnalyticsDB(t)
	repo := postgres.NewOutcomes(pool)
	e := newEvent(t, 1, domain.EventOfferMade, "", base)

	for round := range 10 {
		e.EventID = eventID(100 + round)
		var wg sync.WaitGroup
		results := make([]bool, 2)
		errs := make([]error, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = repo.RecordEvent(ctx, e)
			}()
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("round %d: errors %v", round, errs)
		}
		if results[0] == results[1] {
			t.Fatalf("round %d: recorded=%v, want exactly one true", round, results)
		}
		if n := count(t, ctx, pool, "outcome_events", e.EventID); n != 1 {
			t.Fatalf("round %d: %d rows, want 1", round, n)
		}
	}
}

func TestOutcomes_CountEvents(t *testing.T) {
	ctx, pool := startAnalyticsDB(t)
	repo := postgres.NewOutcomes(pool)
	h := time.Hour
	changed := domain.EventStatusChanged
	for i, e := range []struct {
		eventType, status string
		at                time.Duration
	}{
		{domain.EventJobPublished, "", 0},
		{domain.EventJobPublished, "", h},
		{changed, "SHORTLISTED", h},
		{changed, "SHORTLISTED", 2 * h},
		{changed, "OFFERED", 2 * h},
		{domain.EventOfferMade, "", 3 * h},
		{domain.EventJobClosed, "", 3 * h},
	} {
		mustRecord(t, ctx, repo, newEvent(t, i+1, e.eventType, e.status, base.Add(e.at)))
	}
	at := func(d time.Duration) time.Time { return base.Add(d) }
	all := []domain.EventCount{
		{EventType: changed, NewStatus: "OFFERED", Count: 1},
		{EventType: changed, NewStatus: "SHORTLISTED", Count: 2},
		{EventType: domain.EventJobClosed, Count: 1},
		{EventType: domain.EventJobPublished, Count: 2},
		{EventType: domain.EventOfferMade, Count: 1},
	}

	for _, tt := range []struct {
		name       string
		start, end time.Time
		want       []domain.EventCount
	}{
		{"whole range, sorted by type and status", at(0), at(4 * h), all},
		{"start is inclusive", at(2 * h), at(4 * h), []domain.EventCount{
			{EventType: changed, NewStatus: "OFFERED", Count: 1},
			{EventType: changed, NewStatus: "SHORTLISTED", Count: 1},
			{EventType: domain.EventJobClosed, Count: 1},
			{EventType: domain.EventOfferMade, Count: 1},
		}},
		{"end is exclusive", at(0), at(h), []domain.EventCount{{EventType: domain.EventJobPublished, Count: 1}}},
		{"range with no events", at(5 * h), at(6 * h), nil},
		{"one microsecond window", at(h), at(h + time.Microsecond), []domain.EventCount{
			{EventType: changed, NewStatus: "SHORTLISTED", Count: 1},
			{EventType: domain.EventJobPublished, Count: 1},
		}},
		{"start one nanosecond after an event leaves it out", at(h + time.Nanosecond), at(2 * h), nil},
		{"end one nanosecond after an event keeps it", at(h - time.Minute), at(h + time.Nanosecond), []domain.EventCount{
			{EventType: changed, NewStatus: "SHORTLISTED", Count: 1},
			{EventType: domain.EventJobPublished, Count: 1},
		}},
		{"widest valid range", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC), all},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CountEvents(ctx, mustRange(t, tt.start, tt.end))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

// The database limits back up the domain limits. These events skip
// domain.NewOutcomeEvent on purpose. A refused event must not be marked
// processed, otherwise a redelivery would be skipped and the event lost.
func TestOutcomes_DatabaseLimits(t *testing.T) {
	ctx, pool := startAnalyticsDB(t)
	repo := postgres.NewOutcomes(pool)
	changed := domain.EventStatusChanged

	for i, tt := range []struct {
		name      string
		eventType string
		status    string
		wantErr   bool
	}{
		{"32-character status", changed, strings.Repeat("A", 32), false},
		{"33-character status", changed, strings.Repeat("A", 33), true},
		{"32 two-byte characters", changed, strings.Repeat("é", 32), false},
		{"33 two-byte characters", changed, strings.Repeat("é", 33), true},
		{"status change without a status", changed, "", true},
		{"status on another event type", domain.EventOfferMade, "OFFERED", true},
		{"type analytics does not count", "user.registered", "", true},
		{"empty type", "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := domain.OutcomeEvent{EventID: eventID(200 + i), EventType: tt.eventType, NewStatus: tt.status, OccurredAt: base}
			recorded, err := repo.RecordEvent(ctx, e)
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

	t.Run("empty status string is refused by the CHECK", func(t *testing.T) {
		// RecordEvent turns "" into NULL, so insert directly to test the column CHECK.
		if _, err := pool.Exec(ctx, `INSERT INTO outcome_events (event_id, event_type, new_status, occurred_at) VALUES ($1, $2, '', $3)`,
			eventID(250), changed, base); err == nil {
			t.Fatal("expected the CHECK to refuse an empty status")
		}
	})
	t.Run("event ID that is not a UUID", func(t *testing.T) {
		e := domain.OutcomeEvent{EventID: "event-1", EventType: domain.EventJobPublished, OccurredAt: base}
		if _, err := repo.RecordEvent(ctx, e); err == nil {
			t.Fatal("expected an error for a non-UUID event ID")
		}
	})
	t.Run("a refused event can be recorded on redelivery", func(t *testing.T) {
		e := domain.OutcomeEvent{EventID: eventID(300), EventType: changed, NewStatus: strings.Repeat("A", 33), OccurredAt: base}
		if _, err := repo.RecordEvent(ctx, e); err == nil {
			t.Fatal("expected the first attempt to fail")
		}
		e.NewStatus = "SHORTLISTED"
		mustRecord(t, ctx, repo, e)
	})
}
