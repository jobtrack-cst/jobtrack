//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
)

const (
	reporterID = "11111111-1111-4111-8111-111111111111"
	otherID    = "44444444-4444-4444-8444-444444444444"
	adminID    = "22222222-2222-4222-8222-222222222222"
	unknownID  = "00000000-0000-4000-8000-000000000000"
	jobA       = "a1000000-0000-4000-8000-000000000001"
	jobB       = "a1000000-0000-4000-8000-000000000002"
)

// Microsecond precision, because Postgres stores microseconds.
var base = time.Date(2026, 11, 8, 4, 0, 0, 123456000, time.UTC)

// startModerationDB follows the JT-20 pattern used by company-verification: a
// real Postgres with all migrations applied, run twice to prove they are safe
// on every startup.
func startModerationDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("moderation_test"), pgcontainer.WithUsername("moderation_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

// reportID builds an ID like "b0000000-0000-4000-8000-000000000007".
func reportID(n int) string {
	return fmt.Sprintf("b0000000-0000-4000-8000-%012d", n)
}

func newOpen(n int, job, reporter string, at time.Time) domain.Report {
	return domain.NewReport(reportID(n), job, reporter, "spam", at)
}

func mustCreate(t *testing.T, ctx context.Context, repo *postgres.Reports, r domain.Report) domain.Report {
	t.Helper()
	created, err := repo.Create(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func mustGet(t *testing.T, ctx context.Context, repo *postgres.Reports, id string) domain.Report {
	t.Helper()
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func dismiss(t *testing.T, ctx context.Context, repo *postgres.Reports, r domain.Report, at time.Time) (domain.Report, error) {
	t.Helper()
	if err := r.Dismiss(at); err != nil {
		t.Fatal(err)
	}
	return repo.Dismiss(ctx, r)
}

func removal(job string, at time.Time) domain.Removal {
	return domain.NewRemoval(job, adminID, "fake company", at)
}

func TestReports_CreateAndGet(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)

	want := newOpen(1, jobA, reporterID, base)
	created := mustCreate(t, ctx, repo, want)
	if created != want || created.CreatedAt.Location() != time.UTC {
		t.Fatalf("created %+v, want %+v in UTC", created, want)
	}
	if got := mustGet(t, ctx, repo, want.ID); got != want {
		t.Fatalf("get %+v, want %+v", got, want)
	}

	t.Run("same reporter and job while OPEN", func(t *testing.T) {
		if _, err := repo.Create(ctx, newOpen(2, jobA, reporterID, base)); !errors.Is(err, domain.ErrOpenReportExists) {
			t.Fatalf("err=%v, want ErrOpenReportExists", err)
		}
	})
	t.Run("another reporter on the same job", func(t *testing.T) {
		mustCreate(t, ctx, repo, newOpen(3, jobA, otherID, base))
	})
	t.Run("same reporter on another job", func(t *testing.T) {
		mustCreate(t, ctx, repo, newOpen(4, jobB, reporterID, base))
	})
	t.Run("same reporter again after a dismissal", func(t *testing.T) {
		if _, err := dismiss(t, ctx, repo, want, base.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		mustCreate(t, ctx, repo, newOpen(5, jobA, reporterID, base.Add(time.Hour)))
	})
	t.Run("unknown report", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, unknownID); !errors.Is(err, domain.ErrReportNotFound) {
			t.Fatalf("err=%v, want ErrReportNotFound", err)
		}
	})
	t.Run("a report must start OPEN", func(t *testing.T) {
		bad := newOpen(6, jobB, otherID, base)
		bad.Status = domain.StatusDismissed
		if _, err := repo.Create(ctx, bad); err == nil {
			t.Fatal("expected an error for a non-OPEN new report")
		}
	})
}

// Two identical reports at once: the partial unique index lets exactly one in.
func TestReports_Create_ConcurrentDuplicates(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.Create(ctx, newOpen(i+1, jobA, reporterID, base))
		}()
	}
	wg.Wait()
	wins, losses := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrOpenReportExists):
			losses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d, want 1 and 1", wins, losses)
	}
}

func TestReports_Dismiss(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	later := base.Add(time.Hour)
	r := mustCreate(t, ctx, repo, newOpen(1, jobA, reporterID, base))

	got, err := dismiss(t, ctx, repo, r, later)
	if err != nil || got.Status != domain.StatusDismissed || !got.UpdatedAt.Equal(later) || !got.CreatedAt.Equal(base) {
		t.Fatalf("dismissed %+v,%v, want DISMISSED updated at %v", got, err, later)
	}

	t.Run("dismissing an already dismissed report", func(t *testing.T) {
		if _, err := dismiss(t, ctx, repo, r, later); !errors.Is(err, domain.ErrReportNotOpen) {
			t.Fatalf("err=%v, want ErrReportNotOpen", err)
		}
	})
	t.Run("dismissing an unknown report", func(t *testing.T) {
		missing := newOpen(9, jobA, reporterID, base)
		missing.ID = unknownID
		if _, err := dismiss(t, ctx, repo, missing, later); !errors.Is(err, domain.ErrReportNotFound) {
			t.Fatalf("err=%v, want ErrReportNotFound", err)
		}
	})
	t.Run("dismissing an actioned report", func(t *testing.T) {
		open := mustCreate(t, ctx, repo, newOpen(2, jobB, reporterID, base))
		if err := repo.RemoveJob(ctx, removal(jobB, later)); err != nil {
			t.Fatal(err)
		}
		if _, err := dismiss(t, ctx, repo, open, later); !errors.Is(err, domain.ErrReportNotOpen) {
			t.Fatalf("err=%v, want ErrReportNotOpen", err)
		}
		if got := mustGet(t, ctx, repo, open.ID); got.Status != domain.StatusActioned {
			t.Fatalf("status %v, want ACTIONED to stay", got.Status)
		}
	})
	t.Run("OPEN status is not a dismissal", func(t *testing.T) {
		if _, err := repo.Dismiss(ctx, newOpen(3, jobA, otherID, base)); err == nil {
			t.Fatal("expected an error for an OPEN dismissal")
		}
	})
}

// State transition under concurrency: exactly one of two admins wins.
func TestReports_Dismiss_ConcurrentAdmins(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	r := mustCreate(t, ctx, repo, newOpen(1, jobA, reporterID, base))
	if err := r.Dismiss(base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.Dismiss(ctx, r)
		}()
	}
	wg.Wait()
	wins, losses := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrReportNotOpen):
			losses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d, want 1 and 1", wins, losses)
	}
}

func TestReports_RemoveJob(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	removedAt := base.Add(2 * time.Hour)

	openOne := mustCreate(t, ctx, repo, newOpen(1, jobA, reporterID, base))
	openTwo := mustCreate(t, ctx, repo, newOpen(2, jobA, otherID, base))
	wasDismissed := mustCreate(t, ctx, repo, newOpen(3, jobA, adminID, base))
	if _, err := dismiss(t, ctx, repo, wasDismissed, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	otherJob := mustCreate(t, ctx, repo, newOpen(4, jobB, reporterID, base))

	if err := repo.RemoveJob(ctx, removal(jobA, removedAt)); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name        string
		id          string
		wantStatus  domain.ReportStatus
		wantUpdated time.Time
	}{
		{"first OPEN report is actioned", openOne.ID, domain.StatusActioned, removedAt},
		{"second OPEN report is actioned", openTwo.ID, domain.StatusActioned, removedAt},
		{"dismissed report stays dismissed", wasDismissed.ID, domain.StatusDismissed, base.Add(time.Hour)},
		{"another job's report is untouched", otherJob.ID, domain.StatusOpen, base},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := mustGet(t, ctx, repo, tt.id)
			if got.Status != tt.wantStatus || !got.UpdatedAt.Equal(tt.wantUpdated) {
				t.Fatalf("got %v at %v, want %v at %v", got.Status, got.UpdatedAt, tt.wantStatus, tt.wantUpdated)
			}
		})
	}

	t.Run("removal is stored for the Sprint 2 event", func(t *testing.T) {
		var reason, removedBy string
		var at time.Time
		if err := pool.QueryRow(ctx, `SELECT reason, removed_by, removed_at FROM removed_jobs WHERE job_id = $1`, jobA).Scan(&reason, &removedBy, &at); err != nil {
			t.Fatal(err)
		}
		if reason != "fake company" || removedBy != adminID || !at.Equal(removedAt) {
			t.Fatalf("stored %q %q %v", reason, removedBy, at)
		}
	})
	t.Run("second takedown of the same job", func(t *testing.T) {
		if err := repo.RemoveJob(ctx, removal(jobA, removedAt.Add(time.Hour))); !errors.Is(err, domain.ErrJobAlreadyRemoved) {
			t.Fatalf("err=%v, want ErrJobAlreadyRemoved", err)
		}
	})
	t.Run("reporting a removed job", func(t *testing.T) {
		if _, err := repo.Create(ctx, newOpen(5, jobA, "55555555-5555-4555-8555-555555555555", removedAt)); !errors.Is(err, domain.ErrJobAlreadyRemoved) {
			t.Fatalf("err=%v, want ErrJobAlreadyRemoved", err)
		}
	})
	t.Run("removing a job with no reports", func(t *testing.T) {
		if err := repo.RemoveJob(ctx, removal(unknownID, removedAt)); err != nil {
			t.Fatal(err)
		}
	})
}

// Two admins removing the same job at once: exactly one succeeds.
func TestReports_RemoveJob_ConcurrentAdmins(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = repo.RemoveJob(ctx, removal(jobA, base))
		}()
	}
	wg.Wait()
	wins, losses := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrJobAlreadyRemoved):
			losses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d, want 1 and 1", wins, losses)
	}
}

// A report racing a removal must never be left OPEN on a removed job: it is
// either refused or stored and then actioned.
func TestReports_ReportRacingRemoval_NeverLeavesOpenReport(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)

	for i := range 20 {
		job := fmt.Sprintf("c0000000-0000-4000-8000-%012d", i)
		r := newOpen(i, job, reporterID, base)
		var wg sync.WaitGroup
		var reportErr, removeErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, reportErr = repo.Create(ctx, r) }()
		go func() { defer wg.Done(); removeErr = repo.RemoveJob(ctx, removal(job, base)) }()
		wg.Wait()

		if removeErr != nil {
			t.Fatalf("round %d: remove: %v", i, removeErr)
		}
		switch {
		case errors.Is(reportErr, domain.ErrJobAlreadyRemoved):
		case reportErr == nil:
			if got := mustGet(t, ctx, repo, r.ID); got.Status != domain.StatusActioned {
				t.Fatalf("round %d: report left %v on a removed job", i, got.Status)
			}
		default:
			t.Fatalf("round %d: report: %v", i, reportErr)
		}
	}
}

func TestReports_List(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)
	// Report 1 at base, 2 at +1h (dismissed), 3 at +2h, 4 and 5 both at +3h
	// (same time, so report_id breaks the tie: 5 before 4).
	reporters := []string{reporterID, otherID, adminID, unknownID, "55555555-5555-4555-8555-555555555555"}
	offsets := []time.Duration{0, time.Hour, 2 * time.Hour, 3 * time.Hour, 3 * time.Hour}
	for i := range 5 {
		r := mustCreate(t, ctx, repo, newOpen(i+1, jobA, reporters[i], base.Add(offsets[i])))
		if i == 1 {
			if _, err := dismiss(t, ctx, repo, r, base.Add(4*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}
	ids := func(items []domain.Report) string {
		var b strings.Builder
		for _, v := range items {
			b.WriteString(v.ID[len(v.ID)-1:])
		}
		return b.String()
	}

	for _, tt := range []struct {
		name      string
		status    domain.ReportStatus
		page      domain.Page
		wantIDs   string
		wantTotal int64
	}{
		{"all newest first with ID tie-break", domain.StatusUnspecified, domain.Page{Limit: 20}, "54321", 5},
		{"first page", domain.StatusUnspecified, domain.Page{Limit: 2}, "54", 5},
		{"second page", domain.StatusUnspecified, domain.Page{Limit: 2, Offset: 2}, "32", 5},
		{"last page", domain.StatusUnspecified, domain.Page{Limit: 2, Offset: 4}, "1", 5},
		{"offset past the end keeps the total", domain.StatusUnspecified, domain.Page{Limit: 2, Offset: 10}, "", 5},
		{"open only", domain.StatusOpen, domain.Page{Limit: 20}, "5431", 4},
		{"dismissed only", domain.StatusDismissed, domain.Page{Limit: 20}, "2", 1},
		{"actioned is empty", domain.StatusActioned, domain.Page{Limit: 20}, "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := repo.List(ctx, tt.status, tt.page)
			if err != nil || ids(items) != tt.wantIDs || total != tt.wantTotal {
				t.Fatalf("got ids %q total %d err %v, want %q %d", ids(items), total, err, tt.wantIDs, tt.wantTotal)
			}
		})
	}
}

// The database limits back up the domain limits, counted in characters.
func TestReports_DatabaseLimits(t *testing.T) {
	ctx, pool := startModerationDB(t)
	repo := postgres.NewReports(pool)

	for i, tt := range []struct {
		name    string
		change  func(*domain.Report)
		wantErr bool
	}{
		{"1000-character multibyte reason", func(r *domain.Report) { r.Reason = strings.Repeat("ä", 1000) }, false},
		{"1001-character reason", func(r *domain.Report) { r.Reason = strings.Repeat("ä", 1001) }, true},
		{"empty reason", func(r *domain.Report) { r.Reason = "" }, true},
		{"reporter ID that is not a UUID", func(r *domain.Report) { r.ReporterID = "user-1" }, true},
		{"job ID that is not a UUID", func(r *domain.Report) { r.JobID = "job-1" }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job := fmt.Sprintf("d0000000-0000-4000-8000-%012d", i)
			r := newOpen(i+1, job, reporterID, base)
			tt.change(&r)
			_, err := repo.Create(ctx, r)
			if tt.wantErr != (err != nil) {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}

	for i, tt := range []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{"500-character multibyte removal reason", strings.Repeat("ä", 500), false},
		{"501-character removal reason", strings.Repeat("ä", 501), true},
		{"empty removal reason", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job := fmt.Sprintf("e0000000-0000-4000-8000-%012d", i)
			err := repo.RemoveJob(ctx, domain.NewRemoval(job, adminID, tt.reason, base))
			if tt.wantErr != (err != nil) {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
