//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRepository_PostgresPersistenceIsolationAndConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("application_test"), pgcontainer.WithUsername("application_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanup); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for range 2 {
		if err = db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	repo := postgres.NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, employer, job := uuid.NewString(), uuid.NewString(), uuid.NewString()
	a := domain.Application{ID: uuid.NewString(), ApplicantID: owner, EmployerID: employer, JobID: job, ResumeID: uuid.NewString(), CreatedAt: now, UpdatedAt: now}
	saved, err := repo.Create(ctx, a)
	if err != nil || saved != a {
		t.Fatalf("create mismatch: %v", err)
	}
	got, err := repo.Get(ctx, a.ID)
	if err != nil || got != a {
		t.Fatalf("persist mismatch: %v", err)
	}
	_, err = repo.Get(ctx, uuid.NewString())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	duplicate := a
	duplicate.ID = uuid.NewString()
	_, err = repo.Create(ctx, duplicate)
	if !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	// Exercise the use case with the real repository: duplicate detection is
	// scoped to the authenticated applicant/job, including withdrawn records.
	checkDuplicatePrecedence := func(phase string) {
		t.Run(phase, func(t *testing.T) {
			for _, tt := range []struct {
				name, applicantID, jobID string
				published                bool
				want                     codes.Code
			}{
				{"closed", owner, job, false, codes.AlreadyExists},
				{"expired", owner, job, true, codes.AlreadyExists},
				{"different applicant", uuid.NewString(), job, false, codes.FailedPrecondition},
				{"different job", owner, uuid.NewString(), false, codes.FailedPrecondition},
			} {
				t.Run(tt.name, func(t *testing.T) {
					ctrl := gomock.NewController(t)
					jobs := mocks.NewMockJobPosting(ctrl)
					resumes := mocks.NewMockResume(ctrl)
					requestCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("x-user-id", tt.applicantID, "x-user-role", "applicant"))
					if tt.want != codes.AlreadyExists {
						jobs.EXPECT().GetJob(requestCtx, tt.jobID).Return(domain.Job{ID: tt.jobID, EmployerID: employer, Published: tt.published, Deadline: time.Now().Add(-time.Hour)}, nil)
					}
					service := app.NewService(repo, jobs, resumes)
					_, err := service.Submit(requestCtx, tt.jobID, a.ResumeID)
					if status.Code(err) != tt.want {
						t.Fatalf("code=%v want=%v", status.Code(err), tt.want)
					}
				})
			}
		})
	}
	checkDuplicatePrecedence("submitted duplicate precedence")
	for _, tt := range []struct {
		name string
		f    ports.Filter
		want int64
	}{
		{"applicant", ports.Filter{ApplicantID: owner, Limit: 20}, 1},
		{"other applicant", ports.Filter{ApplicantID: uuid.NewString(), Limit: 20}, 0},
		{"employer", ports.Filter{EmployerID: employer, Limit: 20}, 1},
		{"other employer", ports.Filter{EmployerID: uuid.NewString(), Limit: 20}, 0},
		{"job", ports.Filter{JobID: job, Limit: 20}, 1},
		{"other job", ports.Filter{JobID: uuid.NewString(), Limit: 20}, 0},
		{"admin", ports.Filter{Limit: 20}, 1},
		{"empty later page", ports.Filter{Limit: 20, Offset: 20}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, e := repo.List(ctx, tt.f)
			if e != nil || total != tt.want {
				t.Fatalf("list total=%d err=%v", total, e)
			}
			if tt.f.Offset == 0 && int64(len(rows)) != tt.want {
				t.Fatal("incorrect page size")
			}
			if tt.f.Offset > 0 && len(rows) != 0 {
				t.Fatal("expected empty page")
			}
		})
	}
	_, err = repo.Withdraw(ctx, a.ID, uuid.NewString())
	if !errors.Is(err, domain.ErrWithdrawn) {
		t.Fatal("wrong owner update should fail")
	}
	got, _ = repo.Get(ctx, a.ID)
	if got.Withdrawn {
		t.Fatal("wrong owner changed application")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, e := repo.Withdraw(ctx, a.ID, owner)
			if e == nil {
				successes.Add(1)
			} else if !errors.Is(e, domain.ErrWithdrawn) {
				t.Errorf("withdraw: %v", e)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("withdraw was not atomic")
	}
	got, err = repo.Get(ctx, a.ID)
	if err != nil || !got.Withdrawn || got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatal("withdrawal not persisted")
	}
	// A withdrawn application still reserves its applicant/job pair.
	_, err = repo.Create(ctx, duplicate)
	if !errors.Is(err, domain.ErrDuplicate) {
		t.Fatal("withdrawn duplicate accepted")
	}
	checkDuplicatePrecedence("withdrawn duplicate precedence")
	job = uuid.NewString()
	successes.Store(0)
	for range 8 {
		wg.Go(func() {
			b := a
			b.ID = uuid.NewString()
			b.JobID = job
			_, e := repo.Create(ctx, b)
			if e == nil {
				successes.Add(1)
			} else if !errors.Is(e, domain.ErrDuplicate) {
				t.Errorf("create: %v", e)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("duplicate constraint not atomic")
	}
	rows, total, err := repo.List(ctx, ports.Filter{Limit: 1})
	if err != nil || total != 2 || len(rows) != 1 {
		t.Fatal("pagination failed")
	}
	later, _, err := repo.List(ctx, ports.Filter{Limit: 1, Offset: 1})
	if err != nil || len(later) != 1 || later[0].ID == rows[0].ID {
		t.Fatal("stable ordering failed")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err = repo.Get(canceled, a.ID); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, _, err = repo.List(canceled, ports.Filter{Limit: 1}); err == nil {
		t.Fatal("cancellation ignored")
	}
	pool.Close()
	if _, err = repo.Create(ctx, a); err == nil {
		t.Fatal("closed pool create succeeded")
	}
	if _, err = repo.Withdraw(ctx, a.ID, owner); err == nil {
		t.Fatal("closed pool withdraw succeeded")
	}
}
