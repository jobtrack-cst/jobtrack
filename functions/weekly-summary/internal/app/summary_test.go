package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/app"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/domain"
	"github.com/jobtrack-cst/jobtrack/functions/weekly-summary/internal/ports/mocks"
)

var (
	now          = time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)
	start        = now.Add(-domain.Window)
	errPrivate   = errors.New("private upstream detail")
	wantSubject  = "JobTrack weekly summary (9 Nov 2026 to 16 Nov 2026)"
	statsSummary = domain.Summary{JobsPublished: 3, StatusChanges: []domain.StatusCount{{Status: "SHORTLISTED", Count: 2}}}
)

type fixture struct {
	stats  *mocks.MockStats
	admins *mocks.MockAdmins
	mailer *mocks.MockMailer
	runner app.Runner
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	f := fixture{stats: mocks.NewMockStats(ctrl), admins: mocks.NewMockAdmins(ctrl), mailer: mocks.NewMockMailer(ctrl)}
	f.runner = app.Runner{Stats: f.stats, Admins: f.admins, Mailer: f.mailer,
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return f
}

func TestRunner_Run_SendsOneEmailPerAdminAcrossPages(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.stats.EXPECT().Summary(ctx, start, now).Return(statsSummary, nil)
	gomock.InOrder(
		f.admins.EXPECT().ListAdmins(ctx, "").Return([]string{"a@cst.edu.bt", "b@cst.edu.bt"}, "2", nil),
		f.admins.EXPECT().ListAdmins(ctx, "2").Return([]string{"c@cst.edu.bt"}, "", nil),
	)
	for _, email := range []string{"a@cst.edu.bt", "b@cst.edu.bt", "c@cst.edu.bt"} {
		f.mailer.EXPECT().Send(ctx, email, wantSubject, gomock.Any()).DoAndReturn(
			func(_ context.Context, _, _, body string) error {
				assert.Contains(t, body, "Jobs published: 3\n")
				assert.Contains(t, body, "  SHORTLISTED: 2\n")
				assert.Contains(t, body, "Period: 9 Nov 2026 02:00 UTC to 16 Nov 2026 02:00 UTC")
				return nil
			})
	}
	result, err := f.runner.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, app.Result{Admins: 3, Sent: 3}, result)
}

func TestRunner_Run_ReportsTheRequestedPeriod(t *testing.T) {
	f := newFixture(t)
	echoed := statsSummary
	echoed.Start, echoed.End = time.Unix(0, 0), time.Unix(0, 0)
	f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(echoed, nil)
	f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return([]string{"a@cst.edu.bt"}, "", nil)
	f.mailer.EXPECT().Send(gomock.Any(), "a@cst.edu.bt", wantSubject, gomock.Any()).Return(nil)
	_, err := f.runner.Run(context.Background())
	require.NoError(t, err)
}

func TestRunner_Run_RemovesDuplicateAndBlankEmails(t *testing.T) {
	f := newFixture(t)
	f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
	gomock.InOrder(
		f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return([]string{"Admin@cst.edu.bt", " ", "admin@CST.edu.bt"}, "1", nil),
		f.admins.EXPECT().ListAdmins(gomock.Any(), "1").Return([]string{"ADMIN@cst.edu.bt", ""}, "", nil),
	)
	f.mailer.EXPECT().Send(gomock.Any(), "Admin@cst.edu.bt", wantSubject, gomock.Any()).Return(nil).Times(1)
	result, err := f.runner.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, app.Result{Admins: 1, Sent: 1}, result)
}

func TestRunner_Run_NoAdminsSendsNothing(t *testing.T) {
	f := newFixture(t)
	f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
	f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return(nil, "", nil)
	result, err := f.runner.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, app.Result{}, result)
}

func TestRunner_Run_ReadFailuresSendNothing(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f fixture)
		want  error
	}{
		{"analytics fails", func(f fixture) {
			f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(domain.Summary{}, errPrivate)
		}, app.ErrStats},
		{"auth fails on the first page", func(f fixture) {
			f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
			f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return(nil, "", errPrivate)
		}, app.ErrAdmins},
		{"auth fails on a later page", func(f fixture) {
			f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
			f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return([]string{"a@cst.edu.bt"}, "1", nil)
			f.admins.EXPECT().ListAdmins(gomock.Any(), "1").Return(nil, "", errPrivate)
		}, app.ErrAdmins},
		{"page token repeats", func(f fixture) {
			f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
			f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return([]string{"a@cst.edu.bt"}, "1", nil)
			f.admins.EXPECT().ListAdmins(gomock.Any(), "1").Return([]string{"b@cst.edu.bt"}, "1", nil)
		}, app.ErrAdmins},
		{"too many pages", func(f fixture) {
			f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
			for page := 0; page < app.MaxAdminPages; page++ {
				token := ""
				if page > 0 {
					token = fmt.Sprint(page)
				}
				f.admins.EXPECT().ListAdmins(gomock.Any(), token).Return([]string{"a@cst.edu.bt"}, fmt.Sprint(page+1), nil)
			}
		}, app.ErrAdmins},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f) // no Send expectation: any email fails the test
			result, err := f.runner.Run(context.Background())
			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, app.Result{}, result)
		})
	}
}

func TestRunner_Run_SendFailureTriesTheRestAndFails(t *testing.T) {
	f := newFixture(t)
	f.stats.EXPECT().Summary(gomock.Any(), start, now).Return(statsSummary, nil)
	f.admins.EXPECT().ListAdmins(gomock.Any(), "").Return([]string{"a@cst.edu.bt", "b@cst.edu.bt", "c@cst.edu.bt"}, "", nil)
	gomock.InOrder(
		f.mailer.EXPECT().Send(gomock.Any(), "a@cst.edu.bt", wantSubject, gomock.Any()).Return(nil),
		f.mailer.EXPECT().Send(gomock.Any(), "b@cst.edu.bt", wantSubject, gomock.Any()).Return(errPrivate),
		f.mailer.EXPECT().Send(gomock.Any(), "c@cst.edu.bt", wantSubject, gomock.Any()).Return(nil),
	)
	result, err := f.runner.Run(context.Background())
	require.ErrorIs(t, err, app.ErrSend)
	assert.Equal(t, app.Result{Admins: 3, Sent: 2, Failed: 1}, result)
	assert.Contains(t, err.Error(), "1 of 3")
}
