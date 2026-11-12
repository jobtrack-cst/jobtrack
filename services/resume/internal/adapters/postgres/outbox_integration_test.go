//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func uploaded(t *testing.T, f domain.ResumeFile) *eventsv1.EventEnvelope {
	t.Helper()
	data, err := anypb.New(&eventsv1.ResumeUploaded{ResumeId: f.ID, ApplicantId: f.ApplicantID})
	require.NoError(t, err)
	return &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "resume.uploaded", Producer: "resume", OccurredAt: timestamppb.New(f.CreatedAt), Data: data}
}

type outboxClock struct{}

func (outboxClock) Now() time.Time { return base }

func TestResumes_OutboxAtomicity(t *testing.T) {
	ctx, pool := startResumeDB(t)
	repo := postgres.NewResumes(pool)
	counts := func() (int, int) {
		t.Helper()
		var resumes, outgoing int
		require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM resumes), (SELECT count(*) FROM outgoing_events)`).Scan(&resumes, &outgoing))
		return resumes, outgoing
	}
	f := newFile(applicantA, base, pdf(100))
	event := uploaded(t, f)
	t.Run("success preserves original envelope and pending marker", func(t *testing.T) {
		stored, err := repo.Create(ctx, f, event)
		require.NoError(t, err)
		require.Equal(t, f.Resume, stored)
		var body, eventID, eventType string
		var created time.Time
		var published *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT event_id, event_type, payload, created_at, published_at FROM outgoing_events WHERE event_id=$1`, event.EventId).Scan(&eventID, &eventType, &body, &created, &published))
		original, err := events.MarshalEnvelope(event)
		require.NoError(t, err)
		require.Equal(t, string(original), body)
		decoded, err := events.UnmarshalEnvelope([]byte(body))
		require.NoError(t, err)
		require.True(t, proto.Equal(event, decoded))
		require.Equal(t, event.EventId, eventID)
		require.Equal(t, event.Type, eventType)
		require.Equal(t, base, created.UTC())
		require.Nil(t, published)
		a, b := counts()
		require.Equal(t, 1, a)
		require.Equal(t, 1, b)
	})
	for _, tc := range []struct {
		name string
		edit func(*domain.ResumeFile, *eventsv1.EventEnvelope)
	}{
		{"resume insert failure leaves no outgoing event", func(file *domain.ResumeFile, e *eventsv1.EventEnvelope) { file.FileName = "" }},
		{"outbox duplicate event ID rolls back inserted resume", func(file *domain.ResumeFile, e *eventsv1.EventEnvelope) {
			seed := newFile(applicantA, base, pdf(100))
			seedEvent := uploaded(t, seed)
			_, err := repo.Create(ctx, seed, seedEvent)
			require.NoError(t, err)
			e.EventId = seedEvent.EventId
		}},
		{"mismatched payload rejected", func(file *domain.ResumeFile, e *eventsv1.EventEnvelope) { file.ApplicantID = applicantB }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := newFile(applicantA, base, pdf(100))
			e := uploaded(t, file)
			tc.edit(&file, e)
			a, b := counts()
			got, err := repo.Create(ctx, file, e)
			require.Error(t, err)
			require.Empty(t, got)
			afterA, afterB := counts()
			require.Equal(t, a, afterA)
			require.Equal(t, b, afterB)
			_, err = repo.GetByID(ctx, file.ID)
			require.ErrorIs(t, err, domain.ErrResumeNotFound)
		})
	}
	t.Run("invalid uploads create neither record", func(t *testing.T) {
		svc := app.NewResumes(repo, outboxClock{})
		for _, input := range []domain.UploadInput{
			{FileName: "cv.pdf", ContentType: "application/pdf"},
			{FileName: "cv.pdf", ContentType: "application/pdf", Content: []byte("not PDF")},
			{FileName: "../cv.pdf", ContentType: "application/pdf", Content: pdf(100)},
			{FileName: "cv.pdf", ContentType: "text/plain", Content: pdf(100)},
			{FileName: "cv.pdf", ContentType: "application/pdf", Content: pdf(2_000_001)},
		} {
			a, b := counts()
			_, err := svc.Upload(context.Background(), domain.Caller{UserID: applicantA, Role: domain.RoleApplicant}, input)
			require.Error(t, err)
			afterA, afterB := counts()
			require.Equal(t, a, afterA)
			require.Equal(t, b, afterB)
		}
	})
}
