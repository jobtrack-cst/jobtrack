//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

const (
	employerID = "11111111-1111-4111-8111-111111111111"
	otherID    = "44444444-4444-4444-8444-444444444444"
	adminID    = "22222222-2222-4222-8222-222222222222"
	unknownID  = "00000000-0000-4000-8000-000000000000"
)

// Microsecond precision, because Postgres stores microseconds.
var base = time.Date(2026, 11, 8, 4, 0, 0, 123456000, time.UTC)

// startVerificationDB follows the JT-20 pattern used by auth and employer: a
// real Postgres with all migrations applied, run twice to prove they are safe
// on every startup.
func startVerificationDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("company_verification_test"), pgcontainer.WithUsername("company_verification_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

// newPending builds a request id like "a0000000-0000-4000-8000-00000000000<n>".
func newPending(n int, employer string, at time.Time) domain.VerificationRequest {
	id := "a0000000-0000-4000-8000-" + strings.Repeat("0", 11) + string(rune('0'+n))
	return domain.NewVerificationRequest(id, employer, domain.Submission{RegistrationNumber: "BT-1", Notes: "note"}, at)
}

// eventFor builds the outbox event the app layer would store with v.
func eventFor(t *testing.T, v domain.VerificationRequest) *eventsv1.EventEnvelope {
	t.Helper()
	e, err := app.DecisionEvent(v, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestRequests_CreateGetAndStatuses(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)

	want := newPending(1, employerID, base)
	created, err := repo.Create(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if created != want || created.CreatedAt.Location() != time.UTC {
		t.Fatalf("created %+v, want %+v in UTC", created, want)
	}
	got, err := repo.GetByID(ctx, want.ID)
	if err != nil || got != want {
		t.Fatalf("get %+v,%v, want %+v,nil", got, err, want)
	}
	statuses, err := repo.StatusesByEmployer(ctx, employerID)
	if err != nil || len(statuses) != 1 || statuses[0] != domain.StatusPending {
		t.Fatalf("statuses %v,%v, want [pending]", statuses, err)
	}

	t.Run("second pending request for the same employer", func(t *testing.T) {
		if _, err := repo.Create(ctx, newPending(2, employerID, base)); !errors.Is(err, domain.ErrPendingRequestExists) {
			t.Fatalf("err=%v, want ErrPendingRequestExists", err)
		}
	})
	t.Run("another employer can have a pending request", func(t *testing.T) {
		if _, err := repo.Create(ctx, newPending(3, otherID, base)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unknown request", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, unknownID); !errors.Is(err, domain.ErrRequestNotFound) {
			t.Fatalf("err=%v, want ErrRequestNotFound", err)
		}
	})
	t.Run("employer without requests", func(t *testing.T) {
		statuses, err := repo.StatusesByEmployer(ctx, unknownID)
		if err != nil || len(statuses) != 0 {
			t.Fatalf("statuses %v,%v, want empty", statuses, err)
		}
	})
}

func TestRequests_Decide(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)
	later := base.Add(time.Hour)

	// Reject the first request, then a new one may be created and approved.
	first := newPending(1, employerID, base)
	if _, err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := first.Reject(adminID, "Expired licence", later); err != nil {
		t.Fatal(err)
	}
	rejected, err := repo.Decide(ctx, first, eventFor(t, first))
	if err != nil || rejected != first {
		t.Fatalf("rejected %+v,%v, want %+v,nil", rejected, err, first)
	}

	t.Run("deciding an already decided request", func(t *testing.T) {
		again := newPending(1, employerID, base)
		_ = again.Approve(adminID, later)
		if _, err := repo.Decide(ctx, again, eventFor(t, again)); !errors.Is(err, domain.ErrRequestNotPending) {
			t.Fatalf("err=%v, want ErrRequestNotPending", err)
		}
	})
	t.Run("deciding an unknown request", func(t *testing.T) {
		missing := newPending(9, employerID, base)
		missing.ID = unknownID
		_ = missing.Approve(adminID, later)
		if _, err := repo.Decide(ctx, missing, eventFor(t, missing)); !errors.Is(err, domain.ErrRequestNotFound) {
			t.Fatalf("err=%v, want ErrRequestNotFound", err)
		}
	})

	second := newPending(2, employerID, later)
	if _, err := repo.Create(ctx, second); err != nil {
		t.Fatalf("new request after a rejection: %v", err)
	}
	if err := second.Approve(adminID, later.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	approved, err := repo.Decide(ctx, second, eventFor(t, second))
	if err != nil || approved != second || approved.ReviewedBy != adminID {
		t.Fatalf("approved %+v,%v, want %+v,nil", approved, err, second)
	}

	t.Run("second approval for the same employer", func(t *testing.T) {
		// The app refuses this first; the index is the last line of defence.
		third := newPending(3, employerID, later)
		if _, err := repo.Create(ctx, third); err != nil {
			t.Fatal(err)
		}
		_ = third.Approve(adminID, later)
		if _, err := repo.Decide(ctx, third, eventFor(t, third)); !errors.Is(err, domain.ErrAlreadyVerified) {
			t.Fatalf("err=%v, want ErrAlreadyVerified", err)
		}
	})
	t.Run("pending status is not a decision", func(t *testing.T) {
		approvedLater := newPending(4, otherID, base)
		_ = approvedLater.Approve(adminID, later)
		if _, err := repo.Decide(ctx, newPending(4, otherID, base), eventFor(t, approvedLater)); err == nil {
			t.Fatal("expected an error for a PENDING decision")
		}
	})
}

// State transition under concurrency: exactly one of two admins wins.
func TestRequests_Decide_ConcurrentAdmins(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)
	r := newPending(1, employerID, base)
	if _, err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	approve, reject := r, r
	_ = approve.Approve(adminID, base.Add(time.Minute))
	_ = reject.Reject(adminID, "duplicate", base.Add(time.Minute))

	var wg sync.WaitGroup
	errs := make([]error, 2)
	decisions := []domain.VerificationRequest{approve, reject}
	decisionEvents := []*eventsv1.EventEnvelope{eventFor(t, approve), eventFor(t, reject)}
	for i, v := range decisions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = repo.Decide(ctx, v, decisionEvents[i])
		}()
	}
	wg.Wait()
	wins, losses := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrRequestNotPending):
			losses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d, want 1 and 1", wins, losses)
	}
}

func TestRequests_Lists(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)
	// Employer: rejected at base, rejected at +1h, pending at +2h. Other: pending at +30m.
	for i, at := range []time.Duration{0, time.Hour} {
		r := newPending(i+1, employerID, base.Add(at))
		if _, err := repo.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
		_ = r.Reject(adminID, "missing licence", base.Add(at+time.Minute))
		if _, err := repo.Decide(ctx, r, eventFor(t, r)); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []domain.VerificationRequest{newPending(3, employerID, base.Add(2*time.Hour)), newPending(4, otherID, base.Add(30*time.Minute))} {
		if _, err := repo.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(items []domain.VerificationRequest) string {
		var b strings.Builder
		for _, v := range items {
			b.WriteString(v.ID[len(v.ID)-1:])
		}
		return b.String()
	}

	for _, tt := range []struct {
		name      string
		list      func() ([]domain.VerificationRequest, int64, error)
		wantIDs   string
		wantTotal int64
	}{
		{"own requests newest first", func() ([]domain.VerificationRequest, int64, error) {
			return repo.ListByEmployer(ctx, employerID, domain.Page{Limit: 20})
		}, "321", 3},
		{"own requests first page", func() ([]domain.VerificationRequest, int64, error) {
			return repo.ListByEmployer(ctx, employerID, domain.Page{Limit: 2})
		}, "32", 3},
		{"own requests second page", func() ([]domain.VerificationRequest, int64, error) {
			return repo.ListByEmployer(ctx, employerID, domain.Page{Limit: 2, Offset: 2})
		}, "1", 3},
		{"offset past the end keeps the total", func() ([]domain.VerificationRequest, int64, error) {
			return repo.ListByEmployer(ctx, employerID, domain.Page{Limit: 2, Offset: 10})
		}, "", 3},
		{"other employer sees only own", func() ([]domain.VerificationRequest, int64, error) {
			return repo.ListByEmployer(ctx, otherID, domain.Page{Limit: 20})
		}, "4", 1},
		{"admin list all oldest first", func() ([]domain.VerificationRequest, int64, error) {
			return repo.List(ctx, domain.StatusUnspecified, domain.Page{Limit: 20})
		}, "1423", 4},
		{"admin list pending only", func() ([]domain.VerificationRequest, int64, error) {
			return repo.List(ctx, domain.StatusPending, domain.Page{Limit: 20})
		}, "43", 2},
		{"admin list rejected second page", func() ([]domain.VerificationRequest, int64, error) {
			return repo.List(ctx, domain.StatusRejected, domain.Page{Limit: 1, Offset: 1})
		}, "2", 2},
		{"admin list approved is empty", func() ([]domain.VerificationRequest, int64, error) {
			return repo.List(ctx, domain.StatusApproved, domain.Page{Limit: 20})
		}, "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := tt.list()
			if err != nil || ids(items) != tt.wantIDs || total != tt.wantTotal {
				t.Fatalf("got ids %q total %d err %v, want %q %d", ids(items), total, err, tt.wantIDs, tt.wantTotal)
			}
		})
	}
}

// The database limits back up the domain limits, counted in characters.
func TestRequests_DatabaseLimits(t *testing.T) {
	ctx, pool := startVerificationDB(t)
	repo := postgres.NewRequests(pool)
	employers := []string{
		"b0000000-0000-4000-8000-000000000001", "b0000000-0000-4000-8000-000000000002",
		"b0000000-0000-4000-8000-000000000003", "b0000000-0000-4000-8000-000000000004",
		"b0000000-0000-4000-8000-000000000005", "b0000000-0000-4000-8000-000000000006",
	}
	for i, tt := range []struct {
		name    string
		change  func(*domain.VerificationRequest)
		wantErr bool
	}{
		{"50-character multibyte registration number", func(v *domain.VerificationRequest) { v.RegistrationNumber = strings.Repeat("ä", 50) }, false},
		{"51-character registration number", func(v *domain.VerificationRequest) { v.RegistrationNumber = strings.Repeat("ä", 51) }, true},
		{"empty registration number", func(v *domain.VerificationRequest) { v.RegistrationNumber = "" }, true},
		{"1000-character notes", func(v *domain.VerificationRequest) { v.Notes = strings.Repeat("n", 1000) }, false},
		{"1001-character notes", func(v *domain.VerificationRequest) { v.Notes = strings.Repeat("n", 1001) }, true},
		{"employer ID that is not a UUID", func(v *domain.VerificationRequest) { v.EmployerID = "employer-1" }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := newPending(i+1, employers[i], base)
			tt.change(&v)
			_, err := repo.Create(ctx, v)
			if tt.wantErr != (err != nil) {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}

	t.Run("rejection reason 500 and 501 characters", func(t *testing.T) {
		for i, tt := range []struct {
			n       int
			wantErr bool
		}{{500, false}, {501, true}} {
			v := newPending(7+i, "c0000000-0000-4000-8000-00000000000"+string(rune('1'+i)), base)
			if _, err := repo.Create(ctx, v); err != nil {
				t.Fatal(err)
			}
			_ = v.Reject(adminID, strings.Repeat("ä", tt.n), base.Add(time.Minute))
			if _, err := repo.Decide(ctx, v, eventFor(t, v)); tt.wantErr != (err != nil) {
				t.Fatalf("%d characters: err=%v, wantErr=%v", tt.n, err, tt.wantErr)
			}
		}
	})
}
