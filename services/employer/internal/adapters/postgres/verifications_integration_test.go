//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/employer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
)

var (
	profileAt  = time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC)
	rejectedAt = time.Date(2026, 11, 12, 4, 0, 0, 654321000, time.UTC)
	verifiedAt = time.Date(2026, 11, 12, 4, 30, 0, 123456000, time.UTC)
	appliedAt  = time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)
)

func decision(t *testing.T, n int, employer string, status domain.VerificationStatus, reason string, at time.Time) domain.VerificationDecision {
	t.Helper()
	d, err := domain.NewVerificationDecision(fmt.Sprintf("e0000000-0000-4000-8000-%012d", n), employer, status, reason, at)
	require.NoError(t, err)
	return d
}

func requireProfileStatus(t *testing.T, ctx context.Context, repo *postgres.Profiles, employer string, status domain.VerificationStatus, reason string) domain.CompanyProfile {
	t.Helper()
	got, err := repo.GetByEmployerID(ctx, employer)
	require.NoError(t, err)
	require.Equal(t, status, got.VerificationStatus)
	require.Equal(t, reason, got.RejectionReason)
	return got
}

func processed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventID string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM processed_events WHERE event_id = $1)`, eventID).Scan(&exists))
	return exists
}

func TestVerifications_ApplyToExistingProfile(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	profiles, verifications := postgres.NewProfiles(pool), postgres.NewVerifications(pool)
	_, err := profiles.Create(ctx, newProfile(employerID, "Druk Tech", profileAt))
	require.NoError(t, err)

	rejected := decision(t, 1, employerID, domain.VerificationRejected, "Expired licence", rejectedAt)
	outcome, err := verifications.ApplyVerification(ctx, rejected, appliedAt)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, outcome)
	got := requireProfileStatus(t, ctx, profiles, employerID, domain.VerificationRejected, "Expired licence")
	require.True(t, got.UpdatedAt.Equal(appliedAt))
	require.True(t, got.CreatedAt.Equal(profileAt), "created_at never changes")

	// A later approval replaces the rejection and clears the reason.
	verified := decision(t, 2, employerID, domain.VerificationVerified, "", verifiedAt)
	outcome, err = verifications.ApplyVerification(ctx, verified, appliedAt.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeApplied, outcome)
	requireProfileStatus(t, ctx, profiles, employerID, domain.VerificationVerified, "")

	t.Run("same event again is a duplicate", func(t *testing.T) {
		outcome, err := verifications.ApplyVerification(ctx, verified, appliedAt.Add(2*time.Minute))
		require.NoError(t, err)
		require.Equal(t, domain.OutcomeDuplicate, outcome)
	})
	t.Run("editing the profile keeps the verification", func(t *testing.T) {
		current, err := profiles.GetByEmployerID(ctx, employerID)
		require.NoError(t, err)
		current.CompanyName = "Druk Tech Ltd"
		_, err = profiles.Update(ctx, current)
		require.NoError(t, err)
		requireProfileStatus(t, ctx, profiles, employerID, domain.VerificationVerified, "")
	})
}

// Events can arrive out of order: an older decision never replaces a newer one.
func TestVerifications_OlderDecisionIsStale(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	profiles, verifications := postgres.NewProfiles(pool), postgres.NewVerifications(pool)
	_, err := profiles.Create(ctx, newProfile(employerID, "Druk Tech", profileAt))
	require.NoError(t, err)

	verified := decision(t, 1, employerID, domain.VerificationVerified, "", verifiedAt)
	_, err = verifications.ApplyVerification(ctx, verified, appliedAt)
	require.NoError(t, err)

	older := decision(t, 2, employerID, domain.VerificationRejected, "Expired licence", rejectedAt)
	outcome, err := verifications.ApplyVerification(ctx, older, appliedAt)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeStale, outcome)
	requireProfileStatus(t, ctx, profiles, employerID, domain.VerificationVerified, "")
	require.True(t, processed(t, ctx, pool, older.EventID), "a stale event is still marked processed")

	outcome, err = verifications.ApplyVerification(ctx, older, appliedAt)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeDuplicate, outcome)

	sameTime := decision(t, 3, employerID, domain.VerificationRejected, "Same time", verifiedAt)
	outcome, err = verifications.ApplyVerification(ctx, sameTime, appliedAt)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeStale, outcome, "an equal time keeps the decision stored first")
}

// A decision for an employer with no profile is kept and copied by Create.
func TestVerifications_DecisionBeforeProfileIsCopiedOnCreate(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	profiles, verifications := postgres.NewProfiles(pool), postgres.NewVerifications(pool)

	outcome, err := verifications.ApplyVerification(ctx, decision(t, 1, employerID, domain.VerificationRejected, "Expired licence", rejectedAt), appliedAt)
	require.NoError(t, err)
	require.Equal(t, domain.OutcomeStoredWithoutProfile, outcome)
	_, err = profiles.GetByEmployerID(ctx, employerID)
	require.ErrorIs(t, err, domain.ErrProfileNotFound, "no profile is invented")

	created, err := profiles.Create(ctx, newProfile(employerID, "Druk Tech", profileAt))
	require.NoError(t, err)
	require.Equal(t, domain.VerificationRejected, created.VerificationStatus)
	require.Equal(t, "Expired licence", created.RejectionReason)

	t.Run("other employers still start unverified", func(t *testing.T) {
		created, err := profiles.Create(ctx, newProfile(unknownID, "Other", profileAt))
		require.NoError(t, err)
		require.Equal(t, domain.VerificationUnverified, created.VerificationStatus)
		require.Empty(t, created.RejectionReason)
	})
	t.Run("a second Create is still ALREADY_EXISTS", func(t *testing.T) {
		_, err := profiles.Create(ctx, newProfile(employerID, "Again", profileAt))
		require.ErrorIs(t, err, domain.ErrProfileExists)
	})
}

// A failed write must not mark the event processed, so the retry can apply it.
func TestVerifications_FailedWriteMarksNothingProcessed(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	verifications := postgres.NewVerifications(pool)
	// Bypass the domain check to hit the database limit of 500 characters.
	tooLong := domain.VerificationDecision{
		EventID: "e0000000-0000-4000-8000-000000000001", EmployerID: employerID,
		Status: domain.VerificationRejected, RejectionReason: strings.Repeat("x", 501), DecidedAt: rejectedAt,
	}
	_, err := verifications.ApplyVerification(ctx, tooLong, appliedAt)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "xxx", "driver details are hidden")
	require.False(t, processed(t, ctx, pool, tooLong.EventID))

	_, err = verifications.ApplyVerification(ctx, domain.VerificationDecision{
		EventID: "e0000000-0000-4000-8000-000000000002", EmployerID: employerID,
		Status: domain.VerificationUnverified, DecidedAt: rejectedAt,
	}, appliedAt)
	require.Error(t, err, "unverified is not a decision")
}

// Create and ApplyVerification for the same employer at the same time: the
// employer lock means the decision always ends up in the profile.
func TestVerifications_ConcurrentCreateNeverLosesDecision(t *testing.T) {
	ctx, pool := startEmployerDB(t)
	profiles, verifications := postgres.NewProfiles(pool), postgres.NewVerifications(pool)
	const employers = 20
	var wg sync.WaitGroup
	errs := make(chan error, 2*employers)
	for i := range employers {
		id := fmt.Sprintf("c0000000-0000-4000-8000-%012d", i)
		d := decision(t, i+1, id, domain.VerificationVerified, "", verifiedAt)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := profiles.Create(ctx, newProfile(id, "Concurrent", profileAt))
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := verifications.ApplyVerification(ctx, d, appliedAt)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for i := range employers {
		requireProfileStatus(t, ctx, profiles, fmt.Sprintf("c0000000-0000-4000-8000-%012d", i), domain.VerificationVerified, "")
	}
}
