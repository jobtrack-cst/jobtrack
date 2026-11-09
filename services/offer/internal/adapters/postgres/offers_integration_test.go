//go:build integration

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
)

func syntheticOffer() domain.Offer {
	at := time.Date(2026, 11, 8, 12, 0, 0, 123456000, time.FixedZone("Bhutan", 21600))
	return domain.Offer{ID: uuid.NewString(), ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: uuid.NewString(), Terms: "terms", Status: domain.OfferStatusPending, CreatedAt: at.UTC(), UpdatedAt: at.UTC()}
}
func TestOffers_CreateGetAndDecide(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	repo := postgres.NewOffers(pool)
	for _, next := range []domain.OfferStatus{domain.OfferStatusAccepted, domain.OfferStatusDeclined} {
		t.Run(map[domain.OfferStatus]string{domain.OfferStatusAccepted: "accept", domain.OfferStatusDeclined: "decline"}[next], func(t *testing.T) {
			want := syntheticOffer()
			want.Terms = strings.Repeat("ཀ", 2000)
			got, err := repo.Create(ctx, want)
			if err != nil || got != want {
				t.Fatalf("create %+v %v", got, err)
			}
			got, err = repo.GetByID(ctx, strings.ToUpper(want.ID))
			if err != nil || got != want {
				t.Fatalf("get %+v %v", got, err)
			}
			if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
				t.Fatal("timestamps not UTC")
			}
			duplicate := want
			duplicate.ID = uuid.NewString()
			if _, err := repo.Create(ctx, duplicate); !errors.Is(err, domain.ErrOfferExists) {
				t.Fatal(err)
			}
			if _, err := repo.Decide(ctx, want.ID, uuid.NewString(), next, want.UpdatedAt); !errors.Is(err, domain.ErrPermissionDenied) {
				t.Fatal(err)
			}
			at := want.UpdatedAt.Add(time.Hour)
			got, err = repo.Decide(ctx, want.ID, want.ApplicantID, next, at)
			want.Status = next
			want.UpdatedAt = at
			if err != nil || got != want {
				t.Fatalf("decision %+v %v want %+v", got, err, want)
			}
			for _, again := range []domain.OfferStatus{domain.OfferStatusAccepted, domain.OfferStatusDeclined} {
				if _, err := repo.Decide(ctx, want.ID, want.ApplicantID, again, at); !errors.Is(err, domain.ErrOfferNotPending) {
					t.Fatal(err)
				}
			}
			if _, err := repo.Create(ctx, duplicate); !errors.Is(err, domain.ErrOfferExists) {
				t.Fatal("final offer could be replaced", err)
			}
			persisted, err := repo.GetByID(ctx, want.ID)
			if err != nil || persisted != want {
				t.Fatal(persisted, err)
			}
		})
	}
	if _, err := repo.GetByID(ctx, uuid.NewString()); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, "bad"); !errors.Is(err, domain.ErrInvalidOfferID) {
		t.Fatal(err)
	}
	if _, err := repo.Decide(ctx, uuid.NewString(), uuid.NewString(), domain.OfferStatusAccepted, time.Now()); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.Decide(ctx, uuid.NewString(), uuid.NewString(), domain.OfferStatusPending, time.Now()); !errors.Is(err, domain.ErrOfferNotPending) {
		t.Fatal(err)
	}
	pool.Close()
	o := syntheticOffer()
	for _, operation := range []func() error{
		func() error { _, err := repo.Create(ctx, o); return err },
		func() error { _, err := repo.GetByID(ctx, o.ID); return err },
		func() error {
			_, err := repo.Decide(ctx, o.ID, o.ApplicantID, domain.OfferStatusAccepted, time.Now())
			return err
		},
	} {
		err := operation()
		if err == nil || err.Error() != "offer database operation failed" {
			t.Fatal("unexpected infrastructure error", err)
		}
	}
}

func TestOffers_ConcurrentCreationAndDecisions(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	repo := postgres.NewOffers(pool)
	o := syntheticOffer()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			candidate := o
			candidate.ID = uuid.NewString()
			_, err := repo.Create(ctx, candidate)
			results <- err
		}()
	}
	close(start)
	var successes, duplicates int
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrOfferExists) {
			duplicates++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatalf("successes=%d duplicates=%d", successes, duplicates)
	}
	var storedID string
	if err := pool.QueryRow(ctx, `SELECT offer_id FROM offers WHERE application_id=$1`, o.ApplicationID).Scan(&storedID); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]domain.OfferStatus{{domain.OfferStatusAccepted, domain.OfferStatusDeclined}, {domain.OfferStatusAccepted, domain.OfferStatusAccepted}, {domain.OfferStatusDeclined, domain.OfferStatusDeclined}} {
		candidate := syntheticOffer()
		if _, err := repo.Create(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		start = make(chan struct{})
		for _, next := range pair {
			go func(next domain.OfferStatus) {
				<-start
				_, err := repo.Decide(ctx, candidate.ID, candidate.ApplicantID, next, candidate.UpdatedAt.Add(time.Hour))
				results <- err
			}(next)
		}
		close(start)
		successes = 0
		losers := 0
		for range 2 {
			err := <-results
			if err == nil {
				successes++
			} else if errors.Is(err, domain.ErrOfferNotPending) {
				losers++
			} else {
				t.Fatal(err)
			}
		}
		if successes != 1 || losers != 1 {
			t.Fatalf("successes=%d losers=%d", successes, losers)
		}
	}
}

func TestOffers_MigrationConstraints(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	for _, tc := range []struct {
		name, column string
		value        any
		code         string
	}{
		{"empty terms", "terms", "", "23514"}, {"long terms", "terms", strings.Repeat("ཀ", 2001), "22001"},
		{"invalid status", "status", "other", "23514"},
		{"invalid offer ID", "offer_id", "bad", "22P02"}, {"invalid application ID", "application_id", "bad", "22P02"},
		{"invalid employer ID", "employer_id", "bad", "22P02"}, {"invalid applicant ID", "applicant_id", "bad", "22P02"},
		{"missing offer ID", "offer_id", nil, "23502"}, {"missing application ID", "application_id", nil, "23502"},
		{"missing employer", "employer_id", nil, "23502"}, {"missing applicant", "applicant_id", nil, "23502"},
		{"missing terms", "terms", nil, "23502"}, {"missing status", "status", nil, "23502"},
		{"missing creation", "created_at", nil, "23502"}, {"missing update", "updated_at", nil, "23502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []any{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), "terms", "pending", time.Now(), time.Now()}
			columns := []string{"offer_id", "application_id", "employer_id", "applicant_id", "terms", "status", "created_at", "updated_at"}
			for i, col := range columns {
				if col == tc.column {
					args[i] = tc.value
				}
			}
			_, err := pool.Exec(ctx, `INSERT INTO offers VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("got %v want %s", err, tc.code)
			}
		})
	}
	var foreignKeys int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid='offers'::regclass AND contype='f'`).Scan(&foreignKeys); err != nil || foreignKeys != 0 {
		t.Fatal(foreignKeys, err)
	}
}
