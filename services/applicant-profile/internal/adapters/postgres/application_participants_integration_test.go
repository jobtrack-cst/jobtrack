//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/postgres"
	rabbit "github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/rabbitmq"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	jt114Employer      = "55555555-5555-4555-8555-555555555555"
	jt114OtherEmployer = "66666666-6666-4666-8666-666666666666"
)

func participantRow() domain.ApplicationParticipant {
	return domain.ApplicationParticipant{ApplicationID: uuid.NewString(), ApplicantID: uuid.NewString(), EmployerID: jt114Employer}
}

func storedParticipants(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationID string) []domain.ApplicationParticipant {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT application_id, applicant_id, employer_id FROM application_participants WHERE application_id = $1`, applicationID)
	require.NoError(t, err)
	defer rows.Close()
	var out []domain.ApplicationParticipant
	for rows.Next() {
		var r domain.ApplicationParticipant
		require.NoError(t, rows.Scan(&r.ApplicationID, &r.ApplicantID, &r.EmployerID))
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

func wantSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("got %v, want SQLSTATE %s", err, code)
	}
}

func TestApplicationParticipants_MigrationConstraints(t *testing.T) {
	ctx, pool := startProfileDB(t)

	t.Run("application_participants", func(t *testing.T) {
		id := "77777777-7777-4777-8777-777777777777"
		_, err := pool.Exec(ctx, `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ($1,$1,$1)`, id)
		require.NoError(t, err)
		var createdAt time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT created_at FROM application_participants WHERE application_id = $1`, id).Scan(&createdAt))
		assert.WithinDuration(t, time.Now(), createdAt, time.Minute, "created_at defaults to now()")

		other := "88888888-8888-4888-8888-888888888888"
		for _, tt := range []struct{ name, sql, code string }{
			{"primary key", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ($1,$1,$1)`, "23505"},
			{"application required", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES (NULL,$1,$1)`, "23502"},
			{"applicant required", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ('` + other + `',NULL,$1)`, "23502"},
			{"employer required", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ('` + other + `',$1,NULL)`, "23502"},
			{"created_at required", `INSERT INTO application_participants VALUES ('` + other + `',$1,$1,NULL)`, "23502"},
			{"application UUID", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ('invalid',$1,$1)`, "22P02"},
			{"applicant UUID", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ('` + other + `','invalid',$1)`, "22P02"},
			{"employer UUID", `INSERT INTO application_participants(application_id, applicant_id, employer_id) VALUES ('` + other + `',$1,'invalid')`, "22P02"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				_, err := pool.Exec(ctx, tt.sql, id)
				wantSQLState(t, err, tt.code)
			})
		}
		// No foreign key to applicant_profiles: an applicant without a
		// profile reads as NOT_FOUND instead.
		var fks int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'application_participants'::regclass AND contype = 'f'`).Scan(&fks))
		assert.Zero(t, fks)
	})

	t.Run("processed_events", func(t *testing.T) {
		id := uuid.NewString()
		_, err := pool.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES ($1)`, id)
		require.NoError(t, err)
		var processedAt time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT processed_at FROM processed_events WHERE event_id = $1`, id).Scan(&processedAt))
		assert.WithinDuration(t, time.Now(), processedAt, time.Minute, "processed_at defaults to now()")
		_, err = pool.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES ($1)`, id)
		wantSQLState(t, err, "23505")
		_, err = pool.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES (NULL)`)
		wantSQLState(t, err, "23502")
		_, err = pool.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES ('event-1')`)
		wantSQLState(t, err, "22P02")
	})
}

func TestApplicationParticipants_Record(t *testing.T) {
	ctx, pool := startProfileDB(t)
	repo := postgres.NewApplicationParticipants(pool)

	t.Run("new event stores the row and the claim", func(t *testing.T) {
		row, eventID := participantRow(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, eventID, row))
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("uppercase IDs are stored lowercase", func(t *testing.T) {
		row, eventID := participantRow(), uuid.NewString()
		upper := domain.ApplicationParticipant{ApplicationID: strings.ToUpper(row.ApplicationID),
			ApplicantID: strings.ToUpper(row.ApplicantID), EmployerID: strings.ToUpper(row.EmployerID)}
		require.NoError(t, repo.Record(ctx, strings.ToUpper(eventID), upper))
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("repeated event_id changes nothing", func(t *testing.T) {
		row, eventID := participantRow(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, eventID, row))
		changed := row
		changed.EmployerID = jt114OtherEmployer
		require.NoError(t, repo.Record(ctx, eventID, changed), "a redelivery is acknowledged")
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
	})
	t.Run("same participants under a new event_id are accepted", func(t *testing.T) {
		row, first, second := participantRow(), uuid.NewString(), uuid.NewString()
		require.NoError(t, repo.Record(ctx, first, row))
		require.NoError(t, repo.Record(ctx, second, row))
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.True(t, claimed(t, ctx, pool, second))
	})
	for _, field := range []string{"applicant_id", "employer_id"} {
		t.Run("different "+field+" goes to the dead queue without a claim", func(t *testing.T) {
			row, first, second := participantRow(), uuid.NewString(), uuid.NewString()
			require.NoError(t, repo.Record(ctx, first, row))
			changed := row
			if field == "applicant_id" {
				changed.ApplicantID = uuid.NewString()
			} else {
				changed.EmployerID = jt114OtherEmployer
			}
			err := repo.Record(ctx, second, changed)
			require.ErrorIs(t, err, events.ErrPermanent)
			assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
			assert.False(t, claimed(t, ctx, pool, second), "the claim is rolled back")
		})
	}
	t.Run("invalid IDs are permanent and store nothing", func(t *testing.T) {
		row := participantRow()
		require.ErrorIs(t, repo.Record(ctx, "event-1", row), events.ErrPermanent)
		bad := row
		bad.EmployerID = "employer"
		eventID := uuid.NewString()
		require.ErrorIs(t, repo.Record(ctx, eventID, bad), events.ErrPermanent)
		assert.Empty(t, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
}

func TestApplicationParticipants_GetByApplicationID(t *testing.T) {
	ctx, pool := startProfileDB(t)
	repo := postgres.NewApplicationParticipants(pool)
	row := participantRow()
	require.NoError(t, repo.Record(ctx, uuid.NewString(), row))

	got, err := repo.GetByApplicationID(ctx, row.ApplicationID)
	require.NoError(t, err)
	assert.Equal(t, row, got)

	_, err = repo.GetByApplicationID(ctx, uuid.NewString())
	require.ErrorIs(t, err, domain.ErrApplicationNotFound)

	t.Run("database failure is retryable and hides driver details", func(t *testing.T) {
		pool.Close()
		_, err := repo.GetByApplicationID(ctx, row.ApplicationID)
		require.EqualError(t, err, "get application participants failed")
		err = repo.Record(ctx, uuid.NewString(), participantRow())
		require.EqualError(t, err, "record application participants failed")
		require.NotErrorIs(t, err, events.ErrPermanent, "database errors are retried under JT-61")
	})
}

// startBroker runs a real RabbitMQ for the consumer test.
func startBroker(t *testing.T, ctx context.Context) string {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"},
			Env:        map[string]string{"RABBITMQ_DEFAULT_USER": "consumer_test", "RABBITMQ_DEFAULT_PASS": "synthetic-test-password"},
			WaitingFor: wait.ForListeningPort("5672/tcp"),
		},
		Started: true,
	})
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate RabbitMQ test container failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start RabbitMQ test container failed")
	}
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal("get RabbitMQ host failed")
	}
	port, err := container.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatal("get RabbitMQ port failed")
	}
	return (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("consumer_test", "synthetic-test-password"), Path: "/"}).String()
}

// End to end: application.submitted through the real RabbitMQ and the shared
// events.Consume into Postgres, then the employer reads over the projection.
func TestApplicationConsumer_Integration(t *testing.T) {
	ctx, pool := startProfileDB(t)
	broker := startBroker(t, ctx)
	repo := postgres.NewApplicationParticipants(pool)
	profiles := postgres.NewProfiles(pool)

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
	publish := func(t *testing.T, producer, eventID string, row domain.ApplicationParticipant) []byte {
		t.Helper()
		data, err := anypb.New(&eventsv1.ApplicationSubmitted{ApplicationId: row.ApplicationID, ApplicantId: row.ApplicantID,
			JobId: uuid.NewString(), ResumeId: uuid.NewString(), EmployerId: row.EmployerID})
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

	t.Run("valid event is stored and the owning employer reads the profile and photo", func(t *testing.T) {
		profile := newProfile("jt114")
		require.NoError(t, profiles.Create(ctx, profile))
		row := domain.ApplicationParticipant{ApplicationID: uuid.NewString(), ApplicantID: profile.ApplicantID, EmployerID: jt114Employer}
		eventID := uuid.NewString()
		publish(t, "application", eventID, row)
		publish(t, "application", eventID, row) // a redelivered duplicate is acknowledged
		require.Eventually(t, func() bool { return claimed(t, ctx, pool, eventID) }, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))

		svc := app.NewApplicationParticipants(repo, profiles)
		owner := domain.Caller{UserID: jt114Employer, Role: domain.RoleEmployer}
		_, err := svc.Photo(ctx, owner, row.ApplicationID)
		require.ErrorIs(t, err, domain.ErrPhotoNotFound, "no photo yet")

		photo := domain.Photo{ContentType: domain.PhotoTypeJPEG, Content: dbPhoto(dbJPEG, 64)}
		_, err = profiles.PutPhoto(ctx, profile.ApplicantID, photo, base.Add(time.Hour))
		require.NoError(t, err)
		got, err := svc.Profile(ctx, owner, row.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, profile.ApplicantID, got.ApplicantID)
		assert.Equal(t, profile.Summary, got.Summary)
		assert.True(t, got.HasPhoto, "the profile is read live")
		gotPhoto, err := svc.Photo(ctx, owner, row.ApplicationID)
		require.NoError(t, err)
		assert.Equal(t, photo, gotPhoto)

		_, err = svc.Profile(ctx, domain.Caller{UserID: jt114OtherEmployer, Role: domain.RoleEmployer}, row.ApplicationID)
		require.ErrorIs(t, err, domain.ErrNotApplicationEmployer)
	})
	t.Run("applicant without a profile reads as not found", func(t *testing.T) {
		row, eventID := participantRow(), uuid.NewString()
		publish(t, "application", eventID, row)
		require.Eventually(t, func() bool { return claimed(t, ctx, pool, eventID) }, 10*time.Second, 20*time.Millisecond)
		svc := app.NewApplicationParticipants(repo, profiles)
		_, err := svc.Profile(ctx, domain.Caller{UserID: jt114Employer, Role: domain.RoleEmployer}, row.ApplicationID)
		require.ErrorIs(t, err, domain.ErrProfileNotFound)
	})
	t.Run("wrong producer goes to the dead queue", func(t *testing.T) {
		row, eventID := participantRow(), uuid.NewString()
		dead(t, publish(t, "resume", eventID, row))
		assert.Empty(t, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
	t.Run("invalid ID goes to the dead queue", func(t *testing.T) {
		row := participantRow()
		row.EmployerID = "employer"
		dead(t, publish(t, "application", uuid.NewString(), row))
	})
	t.Run("different participants go to the dead queue without a claim", func(t *testing.T) {
		row := participantRow()
		require.NoError(t, repo.Record(ctx, uuid.NewString(), row))
		changed := row
		changed.EmployerID = jt114OtherEmployer
		eventID := uuid.NewString()
		dead(t, publish(t, "application", eventID, changed))
		assert.Equal(t, []domain.ApplicationParticipant{row}, storedParticipants(t, ctx, pool, row.ApplicationID))
		assert.False(t, claimed(t, ctx, pool, eventID))
	})
}
