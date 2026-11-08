package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports/mocks"
)

const adminID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

func sampleEntries(n int) []domain.Entry {
	base := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	entries := make([]domain.Entry, n)
	for i := range entries {
		entries[i] = domain.Entry{
			EventID:    "8a1c2b3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
			EventType:  "job.published",
			Producer:   "job-posting",
			OccurredAt: base.Add(-time.Duration(i) * time.Minute),
			DataJSON:   `{}`,
		}
	}
	return entries
}

func TestAuditLog_ListEntries_RoleIsCheckedBeforeInput(t *testing.T) {
	tests := []struct {
		name string
		role domain.Role
	}{
		{"applicant", domain.RoleApplicant},
		{"employer", domain.RoleEmployer},
		{"unspecified", domain.RoleUnspecified},
	}
	// Invalid input everywhere: a non-admin must still get the role error.
	bad := app.ListEntriesInput{PageSize: -1, PageToken: "x", EventType: "BAD"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockEntryRepository(ctrl) // no calls expected
			svc := app.NewAuditLog(repo)

			_, err := svc.ListEntries(context.Background(), domain.Caller{UserID: adminID, Role: tt.role}, bad)
			require.ErrorIs(t, err, domain.ErrAdminRoleRequired)
		})
	}
}

func TestAuditLog_ListEntries_RejectsInvalidInput(t *testing.T) {
	t1 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	t0 := t1.Add(-time.Hour)
	tests := []struct {
		name    string
		in      app.ListEntriesInput
		wantErr error
	}{
		{"bad event type", app.ListEntriesInput{EventType: "Job.Published"}, domain.ErrInvalidEventType},
		{"start equal to end", app.ListEntriesInput{Start: &t1, End: &t1}, domain.ErrInvalidTimeRange},
		{"start after end", app.ListEntriesInput{Start: &t1, End: &t0}, domain.ErrInvalidTimeRange},
		{"negative page size", app.ListEntriesInput{PageSize: -1}, domain.ErrInvalidPageSize},
		{"bad page token", app.ListEntriesInput{PageToken: "abc"}, domain.ErrInvalidPageToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockEntryRepository(ctrl) // no calls expected
			svc := app.NewAuditLog(repo)

			_, err := svc.ListEntries(context.Background(), domain.Caller{UserID: adminID, Role: domain.RoleAdmin}, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestAuditLog_ListEntries_PassesFilterAndPage(t *testing.T) {
	t0 := time.Date(2026, 11, 8, 9, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	t1 := t0.Add(time.Hour)
	tests := []struct {
		name       string
		in         app.ListEntriesInput
		wantFilter domain.ListFilter
		wantPage   domain.Page
		returned   int
		total      int64
		wantNext   string
	}{
		{
			name:       "defaults",
			in:         app.ListEntriesInput{},
			wantFilter: domain.ListFilter{},
			wantPage:   domain.Page{Limit: 20, Offset: 0},
			returned:   3, total: 3, wantNext: "",
		},
		{
			name:       "all filters, first of several pages",
			in:         app.ListEntriesInput{PageSize: 2, EventType: "job.published", Start: &t0, End: &t1},
			wantFilter: domain.ListFilter{EventType: "job.published", Start: utc(t0), End: utc(t1)},
			wantPage:   domain.Page{Limit: 2, Offset: 0},
			returned:   2, total: 5, wantNext: "2",
		},
		{
			name:       "page size above maximum is capped",
			in:         app.ListEntriesInput{PageSize: 500, PageToken: "100"},
			wantFilter: domain.ListFilter{},
			wantPage:   domain.Page{Limit: 100, Offset: 100},
			returned:   100, total: 250, wantNext: "200",
		},
		{
			name:       "no matching entries",
			in:         app.ListEntriesInput{EventType: "offer.made"},
			wantFilter: domain.ListFilter{EventType: "offer.made"},
			wantPage:   domain.Page{Limit: 20, Offset: 0},
			returned:   0, total: 0, wantNext: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockEntryRepository(ctrl)
			entries := sampleEntries(tt.returned)
			repo.EXPECT().
				ListEntries(gomock.Any(), tt.wantFilter, tt.wantPage).
				Return(entries, tt.total, nil)
			svc := app.NewAuditLog(repo)

			got, err := svc.ListEntries(context.Background(), domain.Caller{UserID: adminID, Role: domain.RoleAdmin}, tt.in)
			require.NoError(t, err)
			assert.Equal(t, entries, got.Entries)
			assert.Equal(t, tt.total, got.TotalCount)
			assert.Equal(t, tt.wantNext, got.NextPageToken)
		})
	}
}

func TestAuditLog_ListEntries_WrapsRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockEntryRepository(ctrl)
	dbErr := errors.New("connection refused")
	repo.EXPECT().ListEntries(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), dbErr)
	svc := app.NewAuditLog(repo)

	got, err := svc.ListEntries(context.Background(), domain.Caller{UserID: adminID, Role: domain.RoleAdmin}, app.ListEntriesInput{})
	require.ErrorIs(t, err, dbErr)
	assert.Equal(t, app.EntryPage{}, got)
}

func utc(t time.Time) *time.Time {
	u := t.UTC()
	return &u
}
