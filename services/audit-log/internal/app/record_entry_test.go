package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports/mocks"
)

func recValidInput() app.RecordEntryInput {
	return app.RecordEntryInput{
		EventID:    "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAA1",
		EventType:  "job.closed",
		Producer:   "job-posting",
		OccurredAt: time.Date(2026, 11, 13, 15, 0, 0, 0, time.FixedZone("BTT", 6*60*60)),
		DataJSON:   `{"jobId":"33333333-3333-4333-8333-333333333333"}`,
	}
}

func recNewAuditLog(t *testing.T) (*app.AuditLog, *mocks.MockEntryRepository) {
	t.Helper()
	repo := mocks.NewMockEntryRepository(gomock.NewController(t))
	return app.NewAuditLog(repo), repo
}

func TestAuditLog_RecordEntry_StoresValidatedEntry(t *testing.T) {
	auditLog, repo := recNewAuditLog(t)
	repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e domain.Entry) (bool, error) {
		require.Equal(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", e.EventID, "event_id is stored lowercase")
		require.Equal(t, "job.closed", e.EventType)
		require.Equal(t, "job-posting", e.Producer)
		require.Equal(t, time.UTC, e.OccurredAt.Location())
		require.True(t, e.OccurredAt.Equal(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)))
		require.Equal(t, recValidInput().DataJSON, e.DataJSON, "data_json is stored exactly as given")
		return true, nil
	})

	recorded, err := auditLog.RecordEntry(context.Background(), recValidInput())
	require.NoError(t, err)
	require.True(t, recorded)
}

func TestAuditLog_RecordEntry_DuplicateIsNotAnError(t *testing.T) {
	auditLog, repo := recNewAuditLog(t)
	repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).Return(false, nil)

	recorded, err := auditLog.RecordEntry(context.Background(), recValidInput())
	require.NoError(t, err)
	require.False(t, recorded)
}

func TestAuditLog_RecordEntry_RepositoryErrorIsNotInvalidEntry(t *testing.T) {
	auditLog, repo := recNewAuditLog(t)
	errDB := errors.New("database down")
	repo.EXPECT().RecordEntry(gomock.Any(), gomock.Any()).Return(false, errDB)

	_, err := auditLog.RecordEntry(context.Background(), recValidInput())
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, app.ErrInvalidEntry, "a database error must be retried, not dead-lettered")
}

func TestAuditLog_RecordEntry_RejectsInvalidEventsBeforeStoring(t *testing.T) {
	tests := []struct {
		name   string
		change func(in *app.RecordEntryInput)
		want   error
	}{
		{"event ID not a UUID", func(in *app.RecordEntryInput) { in.EventID = "event-1" }, domain.ErrInvalidEventID},
		{"empty event type", func(in *app.RecordEntryInput) { in.EventType = "" }, domain.ErrInvalidEventType},
		{"event type not a routing key", func(in *app.RecordEntryInput) { in.EventType = "JobClosed" }, domain.ErrInvalidEventType},
		{"event type of 101 characters", func(in *app.RecordEntryInput) {
			in.EventType = "a." + strings.Repeat("b", domain.MaxEventTypeLength-1)
		}, domain.ErrInvalidEventType},
		{"producer not kebab-case", func(in *app.RecordEntryInput) { in.Producer = "Job_Posting" }, domain.ErrInvalidProducer},
		{"producer of 51 characters", func(in *app.RecordEntryInput) {
			in.Producer = strings.Repeat("a", domain.MaxProducerLength+1)
		}, domain.ErrInvalidProducer},
		{"missing occurred_at", func(in *app.RecordEntryInput) { in.OccurredAt = time.Time{} }, domain.ErrInvalidOccurredAt},
		{"data not a JSON object", func(in *app.RecordEntryInput) { in.DataJSON = `[]` }, domain.ErrInvalidDataJSON},
		{"data over 65536 bytes", func(in *app.RecordEntryInput) {
			in.DataJSON = `{"v":"` + strings.Repeat("a", domain.MaxDataJSONBytes) + `"}`
		}, domain.ErrInvalidDataJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditLog, _ := recNewAuditLog(t) // the mock fails the test if the repository is called
			in := recValidInput()
			tt.change(&in)
			recorded, err := auditLog.RecordEntry(context.Background(), in)
			require.False(t, recorded)
			require.ErrorIs(t, err, app.ErrInvalidEntry)
			require.ErrorIs(t, err, tt.want)
		})
	}
}
