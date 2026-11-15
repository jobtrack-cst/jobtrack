package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/app"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/ports/mocks"
)

var (
	runAt     = time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)
	windowEnd = runAt.Add(48 * time.Hour)
)

const (
	job1 = "33333333-3333-4333-8333-333333333331"
	job2 = "33333333-3333-4333-8333-333333333332"
	appA = "22222222-2222-4222-8222-22222222222a"
	appB = "22222222-2222-4222-8222-22222222222b"
	appC = "22222222-2222-4222-8222-22222222222c"
)

type fixture struct {
	runner    *app.Runner
	jobs      *mocks.MockJobs
	savers    *mocks.MockSavers
	publisher *mocks.MockPublisher
	published *[]string
}

func newFixture(t *testing.T) fixture {
	ctrl := gomock.NewController(t)
	f := fixture{jobs: mocks.NewMockJobs(ctrl), savers: mocks.NewMockSavers(ctrl), publisher: mocks.NewMockPublisher(ctrl), published: &[]string{}}
	f.runner = &app.Runner{Jobs: f.jobs, Savers: f.savers, Publisher: f.publisher, Now: func() time.Time { return runAt }}
	return f
}

// recordPublishes accepts every publish and records "job/applicant" in order.
func (f fixture) recordPublishes(err func(job, applicant string) error) {
	f.publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, e *eventsv1.EventEnvelope) error {
		if d, ok := ctx.Deadline(); !ok || time.Until(d) > app.PublishTimeout {
			return errors.New("publish without the 10 s deadline")
		}
		var data eventsv1.DeadlineReminder
		if uerr := e.GetData().UnmarshalTo(&data); uerr != nil {
			return uerr
		}
		key := data.GetJobId() + "/" + data.GetApplicantId()
		if err != nil {
			if perr := err(data.GetJobId(), data.GetApplicantId()); perr != nil {
				return perr
			}
		}
		*f.published = append(*f.published, key)
		return nil
	}).AnyTimes()
}

func closing(id string, deadline time.Time) domain.ClosingJob {
	return domain.ClosingJob{ID: id, Deadline: deadline}
}

func TestRun_PublishesOneReminderPerSaverAndJob(t *testing.T) {
	f := newFixture(t)
	d1, d2 := runAt.Add(5*time.Hour), runAt.Add(47*time.Hour)
	gomock.InOrder(
		f.jobs.EXPECT().ListClosing(gomock.Any(), runAt, windowEnd, "").Return([]domain.ClosingJob{closing(job1, d1)}, "1", nil),
		f.jobs.EXPECT().ListClosing(gomock.Any(), runAt, windowEnd, "1").Return([]domain.ClosingJob{closing(job2, d2), closing(job1, d1)}, "", nil),
	)
	gomock.InOrder(
		f.savers.EXPECT().ListSavers(gomock.Any(), job1, "").Return([]string{appA, appB}, "2", nil),
		f.savers.EXPECT().ListSavers(gomock.Any(), job1, "2").Return([]string{appC, appA}, "", nil),
	)
	f.savers.EXPECT().ListSavers(gomock.Any(), job2, "").Return([]string{appB}, "", nil)
	f.recordPublishes(nil)

	res, err := f.runner.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, app.Result{Jobs: 2, Reminders: 4, Published: 4}, res)
	assert.Equal(t, []string{job1 + "/" + appA, job1 + "/" + appB, job1 + "/" + appC, job2 + "/" + appB}, *f.published,
		"both lists paged; a job or saver listed twice is reminded once")
}

func TestRun_NoClosingJobsPublishesNothing(t *testing.T) {
	f := newFixture(t)
	f.jobs.EXPECT().ListClosing(gomock.Any(), runAt, windowEnd, "").Return(nil, "", nil)
	res, err := f.runner.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, app.Result{}, res)
}

func TestRun_EventsAreStableAcrossRuns(t *testing.T) {
	var ids [2][]string
	for i := range ids {
		f := newFixture(t)
		f.runner.Now = func() time.Time { return runAt.Add(time.Duration(i) * 24 * time.Hour) }
		f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").Return([]domain.ClosingJob{closing(job1, runAt.Add(30*time.Hour))}, "", nil)
		f.savers.EXPECT().ListSavers(gomock.Any(), job1, "").Return([]string{appA}, "", nil)
		f.publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *eventsv1.EventEnvelope) error {
			ids[i] = append(ids[i], e.GetEventId())
			assert.Equal(t, "deadline.reminder", e.GetType())
			assert.Equal(t, "deadline-reminder", e.GetProducer())
			return nil
		})
		_, err := f.runner.Run(context.Background())
		require.NoError(t, err)
	}
	assert.Equal(t, ids[0], ids[1], "today's and tomorrow's runs publish the same event_id for the same reminder")
}

func TestRun_JobListFailures(t *testing.T) {
	for name, setup := range map[string]func(f fixture){
		"read fails": func(f fixture) {
			f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").Return(nil, "", errors.New("rpc error: secret detail"))
		},
		"read fails on a later page": func(f fixture) {
			gomock.InOrder(
				f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").Return([]domain.ClosingJob{closing(job1, runAt)}, "1", nil),
				f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "1").Return(nil, "", errors.New("down")),
			)
		},
		"page token repeated": func(f fixture) {
			gomock.InOrder(
				f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").Return(nil, "1", nil),
				f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "1").Return(nil, "1", nil),
			)
		},
		"more than MaxPages pages": func(f fixture) {
			f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, _, _ time.Time, token string) ([]domain.ClosingJob, string, error) {
					n := 0
					fmt.Sscan(token, &n)
					return nil, fmt.Sprint(n + 1), nil
				}).Times(app.MaxPages)
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			setup(f)
			res, err := f.runner.Run(context.Background())
			require.ErrorIs(t, err, app.ErrJobs)
			assert.Equal(t, app.Result{}, res, "nothing is published when the job list is incomplete")
		})
	}
}

func TestRun_SaverAndPublishFailuresDoNotStopOtherJobs(t *testing.T) {
	f := newFixture(t)
	f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").Return(
		[]domain.ClosingJob{closing(job1, runAt.Add(time.Hour)), closing(job2, runAt.Add(2*time.Hour))}, "", nil)
	f.savers.EXPECT().ListSavers(gomock.Any(), job1, "").Return(nil, "", errors.New("saved-jobs down"))
	f.savers.EXPECT().ListSavers(gomock.Any(), job2, "").Return([]string{appA, appB, "not-a-uuid", strings.ToUpper(appC)}, "", nil)
	f.recordPublishes(func(_, applicant string) error {
		if applicant == appB {
			return errors.New("broker down")
		}
		return nil
	})
	res, err := f.runner.Run(context.Background())
	require.ErrorIs(t, err, app.ErrSavers)
	require.ErrorIs(t, err, app.ErrPublish)
	assert.Equal(t, app.Result{Jobs: 2, Reminders: 3, Published: 2, Failed: 2, Skipped: 1}, res)
	assert.Equal(t, []string{job2 + "/" + appA, job2 + "/" + appC}, *f.published, "uppercase saver ID published lowercase")
}

func TestRun_StopsWhenCanceled(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.jobs.EXPECT().ListClosing(gomock.Any(), gomock.Any(), gomock.Any(), "").DoAndReturn(
		func(context.Context, time.Time, time.Time, string) ([]domain.ClosingJob, string, error) {
			cancel()
			return []domain.ClosingJob{closing(job1, runAt.Add(time.Hour))}, "", nil
		})
	_, err := f.runner.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
