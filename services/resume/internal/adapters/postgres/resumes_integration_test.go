//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	applicantA = "11111111-1111-4111-8111-111111111111"
	applicantB = "22222222-2222-4222-8222-222222222222"
	unknownID  = "00000000-0000-4000-8000-000000000000"
)

var base = time.Date(2026, 11, 11, 6, 0, 0, 0, time.UTC)

// startResumeDB follows the job-posting pattern (JT-38): a real Postgres with
// all migrations applied, run twice to prove they are safe on every startup.
func startResumeDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("resume_test"), pgcontainer.WithUsername("resume_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
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

// pdf returns n bytes that start with the PDF signature.
func pdf(n int) []byte {
	return append([]byte("%PDF-"), bytes.Repeat([]byte{0xAB}, n-5)...)
}

func newFile(applicantID string, created time.Time, content []byte) domain.ResumeFile {
	return domain.ResumeFile{
		Resume: domain.Resume{
			ID: uuid.NewString(), ApplicantID: applicantID, FileName: "cv.pdf",
			ContentType: "application/pdf", SizeBytes: int64(len(content)),
			ValidationStatus: domain.ValidationStatusPending, CreatedAt: created, UpdatedAt: created,
		},
		Content: content,
	}
}

func ids(resumes []domain.Resume) []string {
	out := make([]string, len(resumes))
	for i, r := range resumes {
		out[i] = r.ID
	}
	return out
}

func TestResumes_Integration(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewResumes(pool)

	t.Run("create round-trips every field and the content", func(t *testing.T) {
		in := newFile(applicantA, base.Add(123*time.Microsecond), pdf(1000))
		created, err := repo.Create(ctx, in)
		require.NoError(t, err)
		assert.Equal(t, in.Resume, created)

		got, err := repo.GetByID(ctx, in.ID)
		require.NoError(t, err)
		assert.Equal(t, in.Resume, got)
		assert.Equal(t, time.UTC, got.CreatedAt.Location())
		assert.Equal(t, domain.ValidationStatusPending, got.ValidationStatus)
		assert.Equal(t, "", got.RejectionReason)

		file, err := repo.GetFile(ctx, in.ID)
		require.NoError(t, err)
		assert.Equal(t, in.Resume, file.Resume)
		assert.Equal(t, in.Content, file.Content, "binary content is stored byte for byte")
	})

	t.Run("unknown resume is not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, unknownID)
		require.ErrorIs(t, err, domain.ErrResumeNotFound)
		_, err = repo.GetFile(ctx, unknownID)
		require.ErrorIs(t, err, domain.ErrResumeNotFound)
	})

	t.Run("database enforces the limits", func(t *testing.T) {
		setContent := func(f *domain.ResumeFile, content []byte) {
			f.Content, f.SizeBytes = content, int64(len(content))
		}
		tests := []struct {
			name   string
			edit   func(*domain.ResumeFile)
			wantOK bool
		}{
			{"content 2,000,000 bytes", func(f *domain.ResumeFile) { setContent(f, pdf(2_000_000)) }, true},
			{"content 2,000,001 bytes", func(f *domain.ResumeFile) { setContent(f, pdf(2_000_001)) }, false},
			{"file name 255", func(f *domain.ResumeFile) { f.FileName = strings.Repeat("a", 255) }, true},
			{"file name 256", func(f *domain.ResumeFile) { f.FileName = strings.Repeat("a", 256) }, false},
			{"file name 255 multibyte", func(f *domain.ResumeFile) { f.FileName = strings.Repeat("ཀ", 255) }, true},
			{"file name empty", func(f *domain.ResumeFile) { f.FileName = "" }, false},
			{"content type not pdf", func(f *domain.ResumeFile) { f.ContentType = "text/plain" }, false},
			{"size does not match content", func(f *domain.ResumeFile) { f.SizeBytes = 5 }, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// applicantB keeps these rows out of the list test below.
				f := newFile(applicantB, base, pdf(100))
				tt.edit(&f)
				_, err := repo.Create(ctx, f)
				if tt.wantOK {
					require.NoError(t, err)
					file, err := repo.GetFile(ctx, f.ID)
					require.NoError(t, err)
					assert.Equal(t, f.Content, file.Content)
					return
				}
				require.Error(t, err)
				assert.Equal(t, "create resume failed", err.Error(), "driver details are hidden")
			})
		}
	})

	t.Run("invalid status is refused before the database", func(t *testing.T) {
		f := newFile(applicantA, base, pdf(100))
		f.ValidationStatus = domain.ValidationStatusUnspecified
		_, err := repo.Create(ctx, f)
		require.Error(t, err)
	})

	t.Run("list returns only the applicant's resumes, newest first, paged", func(t *testing.T) {
		const applicantC = "33333333-3333-4333-8333-333333333333"
		older := newFile(applicantC, base, pdf(10))
		newer := newFile(applicantC, base.Add(time.Hour), pdf(10))
		// Two resumes with the same created_at are ordered by resume_id descending.
		tieLow := newFile(applicantC, base.Add(2*time.Hour), pdf(10))
		tieHigh := newFile(applicantC, base.Add(2*time.Hour), pdf(10))
		tieLow.ID, tieHigh.ID = "aaaaaaaa-0000-4000-8000-000000000000", "ffffffff-0000-4000-8000-000000000000"
		for _, f := range []domain.ResumeFile{older, newer, tieLow, tieHigh} {
			_, err := repo.Create(ctx, f)
			require.NoError(t, err)
		}

		all, total, err := repo.ListByApplicant(ctx, applicantC, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Equal(t, []string{tieHigh.ID, tieLow.ID, newer.ID, older.ID}, ids(all))

		page2, total, err := repo.ListByApplicant(ctx, applicantC, domain.Page{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, 4, total, "total_count is the full count on every page")
		assert.Equal(t, []string{newer.ID, older.ID}, ids(page2))

		past, total, err := repo.ListByApplicant(ctx, applicantC, domain.Page{Limit: 20, Offset: 10})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Empty(t, past)
	})

	t.Run("applicant with no resumes gets an empty list", func(t *testing.T) {
		got, total, err := repo.ListByApplicant(ctx, unknownID, domain.Page{Limit: 20})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
