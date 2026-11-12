//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidation_IntegrationAtomicResults(t *testing.T) {
	ctx, pool := startResumeDB(t)
	resumes := postgres.NewResumes(pool)
	repo := postgres.NewValidation(pool)
	seed := func() domain.ResumeFile {
		t.Helper()
		f := newFile(applicantA, base, pdf(100))
		_, err := resumes.Create(ctx, f, uploaded(t, f))
		require.NoError(t, err)
		return f
	}
	result := func(f domain.ResumeFile, status domain.ValidationStatus, reason string) domain.ValidationResult {
		return domain.ValidationResult{EventID: uuid.NewString(), ResumeID: f.ID, ApplicantID: f.ApplicantID, Status: status, Reason: reason}
	}
	count := func(id string) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id=$1`, id).Scan(&n))
		return n
	}
	for _, target := range []domain.ValidationStatus{domain.ValidationStatusValidated, domain.ValidationStatusRejected} {
		t.Run(target.String()+" preserves first final and deduplicates", func(t *testing.T) {
			f := seed()
			reason := ""
			if target == domain.ValidationStatusRejected {
				reason = "reject reason"
			}
			e := result(f, target, reason)
			// A validated result clears a stale reason on a PENDING row.
			_, err := pool.Exec(ctx, `UPDATE resumes SET rejection_reason='stale' WHERE resume_id=$1`, f.ID)
			require.NoError(t, err)
			changedAt := base.Add(time.Hour).In(time.FixedZone("UTC+6", 21600))
			got, err := repo.ApplyValidation(ctx, e, changedAt)
			require.NoError(t, err)
			require.Equal(t, domain.ValidationApplied, got)
			require.Equal(t, 1, count(e.EventID))
			stored, err := resumes.GetByID(ctx, f.ID)
			require.NoError(t, err)
			require.Equal(t, target, stored.ValidationStatus)
			require.Equal(t, reason, stored.RejectionReason)
			require.Equal(t, changedAt.UTC(), stored.UpdatedAt)
			got, err = postgres.NewValidation(pool).ApplyValidation(ctx, e, base.Add(2*time.Hour))
			require.NoError(t, err)
			require.Equal(t, domain.ValidationDuplicate, got)
			conflicting := domain.ValidationStatusRejected
			if target == domain.ValidationStatusRejected {
				conflicting = domain.ValidationStatusValidated
			}
			otherReason := ""
			if conflicting == domain.ValidationStatusRejected {
				otherReason = "conflict"
			}
			other := result(f, conflicting, otherReason)
			got, err = repo.ApplyValidation(ctx, other, base.Add(3*time.Hour))
			require.NoError(t, err)
			require.Equal(t, domain.ValidationAlreadyFinal, got)
			require.Equal(t, 1, count(other.EventID))
			same := result(f, target, reason)
			got, err = repo.ApplyValidation(ctx, same, base.Add(4*time.Hour))
			require.NoError(t, err)
			require.Equal(t, domain.ValidationAlreadyFinal, got)
			after, err := resumes.GetByID(ctx, f.ID)
			require.NoError(t, err)
			require.Equal(t, stored, after)
		})
	}
	t.Run("unresolved targets do not claim event IDs", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			edit func(*domain.ValidationResult)
			want error
		}{
			{"unknown", func(e *domain.ValidationResult) { e.ResumeID = uuid.NewString() }, domain.ErrResumeNotFound},
			{"mismatch", func(e *domain.ValidationResult) { e.ApplicantID = applicantB }, domain.ErrValidationApplicantMismatch},
			{"invalid UUID", func(e *domain.ValidationResult) { e.ResumeID = "bad" }, domain.ErrInvalidValidationResult},
			{"invalid reason", func(e *domain.ValidationResult) { e.Reason = "NUL\x00"; e.Status = domain.ValidationStatusRejected }, domain.ErrInvalidValidationResult},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := seed()
				e := result(f, domain.ValidationStatusValidated, "")
				tc.edit(&e)
				got, err := repo.ApplyValidation(ctx, e, base.Add(time.Hour))
				require.ErrorIs(t, err, tc.want)
				require.Equal(t, domain.ValidationOutcomeUnspecified, got)
				require.Zero(t, count(e.EventID))
				after, err := resumes.GetByID(ctx, f.ID)
				require.NoError(t, err)
				require.Equal(t, f.Resume, after)
			})
		}
	})
	t.Run("update failure rolls back event claim", func(t *testing.T) {
		f := seed()
		e := result(f, domain.ValidationStatusValidated, "")
		// Constraint applies only to this synthetic row in a disposable DB.
		_, err := pool.Exec(ctx, `ALTER TABLE resumes ADD CONSTRAINT test_validation_failure CHECK (resume_id <> '`+f.ID+`'::uuid OR validation_status='pending')`)
		require.NoError(t, err)
		got, err := repo.ApplyValidation(ctx, e, base.Add(time.Hour))
		require.Error(t, err)
		require.Equal(t, "apply validation result failed", err.Error())
		require.Equal(t, domain.ValidationOutcomeUnspecified, got)
		require.Zero(t, count(e.EventID))
		after, err := resumes.GetByID(ctx, f.ID)
		require.NoError(t, err)
		require.Equal(t, f.Resume, after)
		_, err = pool.Exec(ctx, `ALTER TABLE resumes DROP CONSTRAINT test_validation_failure`)
		require.NoError(t, err)
		got, err = repo.ApplyValidation(ctx, e, base.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, domain.ValidationApplied, got)
	})
	t.Run("commit failure rolls back both writes", func(t *testing.T) {
		f := seed()
		e := result(f, domain.ValidationStatusRejected, "reason")
		_, err := pool.Exec(ctx, `CREATE FUNCTION test_fail_validation_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic commit failure'; END $$`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `CREATE CONSTRAINT TRIGGER test_validation_commit AFTER INSERT ON processed_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION test_fail_validation_commit()`)
		require.NoError(t, err)
		got, err := repo.ApplyValidation(ctx, e, base.Add(time.Hour))
		require.Error(t, err)
		require.Equal(t, domain.ValidationOutcomeUnspecified, got)
		require.Zero(t, count(e.EventID))
		after, err := resumes.GetByID(ctx, f.ID)
		require.NoError(t, err)
		require.Equal(t, f.Resume, after)
		_, err = pool.Exec(ctx, `DROP TRIGGER test_validation_commit ON processed_events`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DROP FUNCTION test_fail_validation_commit()`)
		require.NoError(t, err)
		got, err = repo.ApplyValidation(ctx, e, base.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, domain.ValidationApplied, got)
	})
	for _, sameID := range []bool{false, true} {
		name := "different IDs"
		if sameID {
			name = "same ID"
		}
		t.Run("concurrent "+name, func(t *testing.T) {
			f := seed()
			const n = 12
			entries := make([]domain.ValidationResult, n)
			for i := range entries {
				status := domain.ValidationStatusValidated
				reason := ""
				if i%2 == 1 {
					status = domain.ValidationStatusRejected
					reason = "rejected"
				}
				entries[i] = result(f, status, reason)
			}
			if sameID {
				for i := range entries {
					entries[i] = entries[0]
				}
			}
			type outcome struct {
				index int
				value domain.ValidationOutcome
				err   error
			}
			outcomes := make(chan outcome, n)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range entries {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					o, err := repo.ApplyValidation(ctx, entries[i], base.Add(time.Duration(i+1)*time.Hour))
					outcomes <- outcome{i, o, err}
				}(i)
			}
			close(start)
			wg.Wait()
			close(outcomes)
			applied := 0
			winner := -1
			for o := range outcomes {
				require.NoError(t, o.err)
				if o.value == domain.ValidationApplied {
					applied++
					winner = o.index
				} else if sameID {
					require.Equal(t, domain.ValidationDuplicate, o.value)
				} else {
					require.Equal(t, domain.ValidationAlreadyFinal, o.value)
				}
			}
			require.Equal(t, 1, applied)
			stored, err := resumes.GetByID(ctx, f.ID)
			require.NoError(t, err)
			require.Equal(t, entries[winner].Status, stored.ValidationStatus)
			require.Equal(t, entries[winner].Reason, stored.RejectionReason)
			require.Equal(t, base.Add(time.Duration(winner+1)*time.Hour), stored.UpdatedAt)
			for _, e := range entries {
				require.Equal(t, 1, count(e.EventID))
			}
		})
	}
	t.Run("cancellation records nothing", func(t *testing.T) {
		f := seed()
		e := result(f, domain.ValidationStatusValidated, "")
		c, cancel := context.WithCancel(ctx)
		cancel()
		_, err := repo.ApplyValidation(c, e, base)
		require.ErrorIs(t, err, context.Canceled)
		require.Zero(t, count(e.EventID))
	})
}
