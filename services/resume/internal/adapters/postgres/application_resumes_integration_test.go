//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	rabbit "github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	jt114Employer      = "55555555-5555-4555-8555-555555555555"
	jt114OtherEmployer = "66666666-6666-4666-8666-666666666666"
)

func applicationRow() domain.ApplicationResume {
	return domain.ApplicationResume{ApplicationID: uuid.NewString(), ApplicantID: applicantA, EmployerID: jt114Employer, ResumeID: uuid.NewString()}
}

func storedApplicationRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationID string) []domain.ApplicationResume {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT application_id, applicant_id, employer_id, resume_id FROM application_resumes WHERE application_id = $1`, applicationID)
	require.NoError(t, err)
	defer rows.Close()
	var out []domain.ApplicationResume
	for rows.Next() {
		var r domain.ApplicationResume
		require.NoError(t, rows.Scan(&r.ApplicationID, &r.ApplicantID, &r.EmployerID, &r.ResumeID))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func claimed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventID string) bool {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id = $1`, eventID).Scan(&n))
	return n == 1
}

func TestApplicationResumes_MigrationConstraints(t *testing.T) {
	ctx, pool := startResumeDB(t)
	id := "77777777-7777-4777-8777-777777777777"
	_, err := pool.Exec(ctx, `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ($1,$1,$1,$1)`, id)
	require.NoError(t, err)
	var createdAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT created_at FROM application_resumes WHERE application_id = $1`, id).Scan(&createdAt))
	assert.WithinDuration(t, time.Now(), createdAt, time.Minute, "created_at defaults to now()")

	other := "88888888-8888-4888-8888-888888888888"
	for _, tt := range []struct{ name, sql, code string }{
		{"primary key", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ($1,$1,$1,$1)`, "23505"},
		{"application required", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES (NULL,$1,$1,$1)`, "23502"},
		{"applicant required", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `',NULL,$1,$1)`, "23502"},
		{"employer required", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `',$1,NULL,$1)`, "23502"},
		{"resume required", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `',$1,$1,NULL)`, "23502"},
		{"created_at required", `INSERT INTO application_resumes VALUES ('` + other + `',$1,$1,$1,NULL)`, "23502"},
		{"application UUID", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('invalid',$1,$1,$1)`, "22P02"},
		{"applicant UUID", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `','invalid',$1,$1)`, "22P02"},
		{"employer UUID", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `',$1,'invalid',$1)`, "22P02"},
		{"resume UUID", `INSERT INTO application_resumes(application_id, applicant_id, employer_id, resume_id) VALUES ('` + other + `',$1,$1,'invalid')`, "22P02"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.sql, id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tt.code {
				t.Fatalf("got %v, want SQLSTATE %s", err, tt.code)
			}
		})
	}
	// No foreign key to resumes: a missing resume reads as NOT_FOUND instead.
	var fks int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'application_resumes'::regclass AND contype = 'f'`).Scan(&fks))
	assert.Zero(t, fks)
}

func TestApplicationResumes_Record(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewApplicationResumes(pool)

	t.Run("new event stores the row and the claim", func(t *testing.T) {
		row, eventID := applicationRow(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, eventID, row))
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("uppercase IDs are stored lowercase", func(t *testing.T) {
		row, eventID := applicationRow(), uuid.NewString()
		upper := domain.ApplicationResume{ApplicationID: strings.ToUpper(row.ApplicationID), ApplicantID: strings.ToUpper(row.ApplicantID),
			EmployerID: strings.ToUpper(row.EmployerID), ResumeID: strings.ToUpper(row.ResumeID)}
		require.NoError(t, repo.Record(ctx, strings.ToUpper(eventID), upper))
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("repeated event_id changes nothing", func(t *testing.T) {
		row, eventID := applicationRow(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, eventID, row))
		changed := row
		changed.EmployerID = jt114OtherEmployer
		require.NoError(t, repo.Record(ctx, eventID, changed), "a redelivery is acknowledged")
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
	})
	t.Run("same participants under a new event_id are accepted", func(t *testing.T) {
		row, first, second := applicationRow(), uuid.NewString(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, first, row))
		require.NoError(t, repo.Record(ctx, second, row))
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, second))
	})
	for _, field := range []string{"applicant_id", "employer_id", "resume_id"} {
		t.Run("different "+field+" goes to the dead queue without a claim", func(t *testing.T) {
			row, first, second := applicationRow(), uuid.NewString(), uuid.NewString()
			require.NoError(t, repo.Record(ctx, first, row))
			changed := row
			switch field {
			case "applicant_id":
				changed.ApplicantID = applicantB
			case "employer_id":
				changed.EmployerID = jt114OtherEmployer
			case "resume_id":
				changed.ResumeID = uuid.NewString()
			}
			err := repo.Record(ctx, second, changed)
			require.ErrorIs(t, err, events.ErrPermanent)
			assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
			assert.False(t, claimed(t, ctx, pool, second), "the claim is rolled back")
		})
	}
	t.Run("invalid IDs are permanent and store nothing", func(t *testing.T) {
		row := applicationRow()
		require.ErrorIs(t, repo.Record(ctx, "event-1", row), events.ErrPermanent)
		bad := row
		bad.ResumeID = "resume"
		eventID := uuid.NewString()
		require.ErrorIs(t, repo.Record(ctx, eventID, bad), events.ErrPermanent)
		assert.Empty(t, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
}

func TestApplicationResumes_GetByApplicationID(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewApplicationResumes(pool)
	row := applicationRow()
	require.NoError(t, repo.Record(ctx, uuid.NewString(), row))

	got, err := repo.GetByApplicationID(ctx, row.ApplicationID)
	require.NoError(t, err)
	assert.Equal(t, row, got)

	_, err = repo.GetByApplicationID(ctx, unknownID)
	require.ErrorIs(t, err, domain.ErrApplicationNotFound)

	t.Run("database failure is retryable and hides driver details", func(t *testing.T) {
		pool.Close()
		_, err := repo.GetByApplicationID(ctx, row.ApplicationID)
		require.EqualError(t, err, "get application resume failed")
		err = repo.Record(ctx, uuid.NewString(), applicationRow())
		require.EqualError(t, err, "record application resume failed")
		require.NotErrorIs(t, err, events.ErrPermanent, "database errors are retried under JT-61")
	})
}

// End to end: application.submitted through the real RabbitMQ and the shared
// events.Consume into Postgres, then the employer download over the projection.
func TestApplicationConsumer_Integration(t *testing.T) {
	ctx, pool := startResumeDB(t)
	broker := dispatcherBroker(t, ctx)
	repo := postgres.NewApplicationResumes(pool)
	resumes := postgres.NewResumes(pool)

	cctx, cancel := context.WithCancel(ctx)
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	cfg := rabbit.ApplicationConsumerConfig(broker, logging.New("test", io.Discard), repo)
	cfg.OnReady = func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	go func() { done <- events.Consume(cctx, cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("consumer did not stop")
		}
	})
	select {
	case <-ready:
	case <-time.After(30 * time.Second):
		t.Fatal("consumer did not become ready")
	}

	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	require.NoError(t, err)
	require.NoError(t, ch.Confirm(false))
	publish := func(t *testing.T, producer, eventID string, row domain.ApplicationResume) []byte {
		t.Helper()
		data, err := anypb.New(&eventsv1.ApplicationSubmitted{ApplicationId: row.ApplicationID, ApplicantId: row.ApplicantID,
			JobId: uuid.NewString(), ResumeId: row.ResumeID, EmployerId: row.EmployerID})
		require.NoError(t, err)
		body, err := events.MarshalEnvelope(&eventsv1.EventEnvelope{EventId: eventID, Type: "application.submitted",
			Producer: producer, OccurredAt: timestamppb.New(base), Data: data})
		require.NoError(t, err)
		confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "jobtrack.events", "application.submitted", true, false,
			amqp.Publishing{Body: body, DeliveryMode: amqp.Persistent, ContentType: "application/json"})
		require.NoError(t, err)
		ok, err := confirm.WaitContext(ctx)
		require.NoError(t, err)
		require.True(t, ok)
		return body
	}
	dead := func(t *testing.T, want []byte) {
		t.Helper()
		var got amqp.Delivery
		require.Eventually(t, func() bool {
			m, ok, err := ch.Get(rabbit.ApplicationQueue+".dead", false)
			if err == nil && ok {
				got = m
				return true
			}
			return false
		}, 15*time.Second, 20*time.Millisecond)
		require.Equal(t, want, got.Body)
		require.NoError(t, got.Ack(false))
	}

	t.Run("valid event is stored and the owning employer can download", func(t *testing.T) {
		f := newFile(applicantA, base, pdf(100))
		_, err := resumes.Create(ctx, f, uploaded(t, f))
		require.NoError(t, err)
		row := domain.ApplicationResume{ApplicationID: uuid.NewString(), ApplicantID: applicantA, EmployerID: jt114Employer, ResumeID: f.ID}
		eventID := uuid.NewString()
		publish(t, "application", eventID, row)
		publish(t, "application", eventID, row) // a redelivered duplicate is acknowledged
		require.Eventually(t, func() bool { return claimed(t, ctx, pool, eventID) }, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))

		svc := app.NewApplicationResumes(repo, resumes)
		file, err := svc.Download(ctx, domain.Caller{UserID: jt114Employer, Role: domain.RoleEmployer}, row.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, f.Content, file.Content)
		_, err = svc.Download(ctx, domain.Caller{UserID: jt114OtherEmployer, Role: domain.RoleEmployer}, row.ApplicationID)
		require.ErrorIs(t, err, domain.ErrNotApplicationEmployer)
	})
	t.Run("wrong producer goes to the dead queue", func(t *testing.T) {
		row, eventID := applicationRow(), uuid.NewString()
		dead(t, publish(t, "resume", eventID, row))
		assert.Empty(t, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("invalid ID goes to the dead queue", func(t *testing.T) {
		row := applicationRow()
		row.EmployerID = "employer"
		dead(t, publish(t, "application", uuid.NewString(), row))
	})
	t.Run("different participants go to the dead queue without a claim", func(t *testing.T) {
		row := applicationRow()
		require.NoError(t, repo.Record(ctx, uuid.NewString(), row))
		changed := row
		changed.EmployerID = jt114OtherEmployer
		eventID := uuid.NewString()
		dead(t, publish(t, "application", eventID, changed))
		assert.Equal(t, []domain.ApplicationResume{row}, storedApplicationRows(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
}
