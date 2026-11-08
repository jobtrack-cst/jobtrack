package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/ports/mocks"
)

const adminID = "3f2b8c1e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"

var (
	bt    = time.FixedZone("BTT", 6*3600)
	start = time.Date(2026, 11, 2, 0, 0, 0, 0, bt)
	end   = time.Date(2026, 11, 9, 0, 0, 0, 0, bt)
	admin = domain.Caller{UserID: adminID, Role: domain.RoleAdmin}
)

func ptr(t time.Time) *time.Time { return &t }

func newService(t *testing.T) (*app.Analytics, *mocks.MockOutcomeRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOutcomeRepository(ctrl)
	return app.NewAnalytics(repo), repo
}

// Invalid input for both RPCs. No repository call is expected for any of them.
var invalidInputs = []struct {
	name    string
	in      app.SummaryInput
	wantErr error
}{
	{"no times", app.SummaryInput{}, domain.ErrInvalidTimestamp},
	{"missing start", app.SummaryInput{End: ptr(end)}, domain.ErrInvalidTimestamp},
	{"missing end", app.SummaryInput{Start: ptr(start)}, domain.ErrInvalidTimestamp},
	{"start equal to end", app.SummaryInput{Start: ptr(start), End: ptr(start)}, domain.ErrInvalidTimeRange},
	{"start after end", app.SummaryInput{Start: ptr(end), End: ptr(start)}, domain.ErrInvalidTimeRange},
}

func TestAnalytics_GetSummary_RoleIsCheckedBeforeInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		role domain.Role
	}{
		{"applicant", domain.RoleApplicant},
		{"employer", domain.RoleEmployer},
		{"unspecified", domain.RoleUnspecified},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newService(t) // no calls expected
			caller := domain.Caller{UserID: adminID, Role: tt.role}
			// Valid and invalid input alike: a non-admin always gets the role error.
			inputs := []app.SummaryInput{{Start: ptr(start), End: ptr(end)}}
			for _, bad := range invalidInputs {
				inputs = append(inputs, bad.in)
			}
			for i, in := range inputs {
				got, err := svc.GetSummary(context.Background(), caller, in)
				require.ErrorIs(t, err, domain.ErrAdminRoleRequired, "input %d", i)
				assert.Equal(t, domain.Summary{}, got)
			}
		})
	}
}

func TestAnalytics_RejectsInvalidInput(t *testing.T) {
	for _, tt := range invalidInputs {
		t.Run("GetSummary "+tt.name, func(t *testing.T) {
			svc, _ := newService(t)
			_, err := svc.GetSummary(context.Background(), admin, tt.in)
			require.ErrorIs(t, err, tt.wantErr)
		})
		t.Run("GetInternalSummary "+tt.name, func(t *testing.T) {
			svc, _ := newService(t)
			_, err := svc.GetInternalSummary(context.Background(), tt.in)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestAnalytics_BothSummariesCountTheSameRange(t *testing.T) {
	rows := []domain.EventCount{
		{EventType: domain.EventJobPublished, Count: 3},
		{EventType: domain.EventStatusChanged, NewStatus: "SHORTLISTED", Count: 2},
		{EventType: domain.EventStatusChanged, NewStatus: "OFFERED", Count: 1},
	}
	want := domain.Summary{JobsPublished: 3, StatusChanges: []domain.StatusCount{
		{Status: "OFFERED", Count: 1}, {Status: "SHORTLISTED", Count: 2},
	}}
	// The repository gets the range in UTC.
	wantRange := domain.TimeRange{Start: start.UTC(), End: end.UTC()}
	in := app.SummaryInput{Start: ptr(start), End: ptr(end)}
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		call func(*app.Analytics) (domain.Summary, error)
	}{
		{"GetSummary as admin", func(a *app.Analytics) (domain.Summary, error) { return a.GetSummary(ctx, admin, in) }},
		{"GetInternalSummary", func(a *app.Analytics) (domain.Summary, error) { return a.GetInternalSummary(ctx, in) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)
			repo.EXPECT().CountEvents(ctx, wantRange).Return(rows, nil).Times(1)
			got, err := tt.call(svc)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestAnalytics_EmptyRangeGivesZeros(t *testing.T) {
	svc, repo := newService(t)
	repo.EXPECT().CountEvents(gomock.Any(), gomock.Any()).Return(nil, nil)
	got, err := svc.GetSummary(context.Background(), admin, app.SummaryInput{Start: ptr(start), End: ptr(end)})
	require.NoError(t, err)
	assert.Equal(t, domain.Summary{}, got)
}

func TestAnalytics_WrapsRepositoryError(t *testing.T) {
	dbErr := errors.New("connection refused")
	in := app.SummaryInput{Start: ptr(start), End: ptr(end)}
	for _, tt := range []struct {
		name string
		call func(*app.Analytics) (domain.Summary, error)
	}{
		{"GetSummary", func(a *app.Analytics) (domain.Summary, error) { return a.GetSummary(context.Background(), admin, in) }},
		{"GetInternalSummary", func(a *app.Analytics) (domain.Summary, error) { return a.GetInternalSummary(context.Background(), in) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)
			repo.EXPECT().CountEvents(gomock.Any(), gomock.Any()).Return(nil, dbErr)
			got, err := tt.call(svc)
			require.ErrorIs(t, err, dbErr)
			assert.Equal(t, domain.Summary{}, got)
		})
	}
}
