package clients_test

import (
	"context"
	"errors"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/functions/deadline-reminder/internal/domain"
)

type fakeJobPosting struct {
	got  *jobpostingv1.ListJobsClosingBetweenRequest
	resp *jobpostingv1.ListJobsClosingBetweenResponse
	err  error
}

func (f *fakeJobPosting) ListJobsClosingBetween(_ context.Context, in *jobpostingv1.ListJobsClosingBetweenRequest, _ ...grpc.CallOption) (*jobpostingv1.ListJobsClosingBetweenResponse, error) {
	f.got = in
	return f.resp, f.err
}

type fakeSavedJobs struct {
	got  *savedjobsv1.ListJobSaversRequest
	resp *savedjobsv1.ListJobSaversResponse
	err  error
}

func (f *fakeSavedJobs) ListJobSavers(_ context.Context, in *savedjobsv1.ListJobSaversRequest, _ ...grpc.CallOption) (*savedjobsv1.ListJobSaversResponse, error) {
	f.got = in
	return f.resp, f.err
}

var (
	start    = time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)
	end      = start.Add(48 * time.Hour)
	deadline = start.Add(10 * time.Hour)
)

func TestJobPosting_ListClosing(t *testing.T) {
	api := &fakeJobPosting{resp: &jobpostingv1.ListJobsClosingBetweenResponse{
		Jobs: []*jobpostingv1.Job{
			{JobId: "job-1", Title: "ignored", Deadline: timestamppb.New(deadline)},
			{JobId: "job-2"},
			{JobId: "job-3", Deadline: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}},
		},
		Pagination: &commonv1.PaginationResponse{NextPageToken: "100", TotalCount: 150},
	}}
	jobs, next, err := clients.JobPosting{API: api}.ListClosing(context.Background(), start, end, "token")
	require.NoError(t, err)
	assert.Equal(t, start, api.got.GetStartTime().AsTime())
	assert.Equal(t, end, api.got.GetEndTime().AsTime())
	assert.Equal(t, int32(100), api.got.GetPagination().GetPageSize())
	assert.Equal(t, "token", api.got.GetPagination().GetPageToken())
	assert.Equal(t, "100", next)
	assert.Equal(t, []domain.ClosingJob{{ID: "job-1", Deadline: deadline}, {ID: "job-2"}, {ID: "job-3"}}, jobs,
		"a missing or invalid deadline becomes zero, and the reminder is then skipped")

	_, _, err = clients.JobPosting{API: &fakeJobPosting{err: errors.New("unavailable")}}.ListClosing(context.Background(), start, end, "")
	require.Error(t, err)
}

func TestSavedJobs_ListSavers(t *testing.T) {
	api := &fakeSavedJobs{resp: &savedjobsv1.ListJobSaversResponse{
		ApplicantIds: []string{"a", "b"},
		Pagination:   &commonv1.PaginationResponse{NextPageToken: "200"},
	}}
	ids, next, err := clients.SavedJobs{API: api}.ListSavers(context.Background(), "job-1", "100")
	require.NoError(t, err)
	assert.Equal(t, "job-1", api.got.GetJobId())
	assert.Equal(t, int32(100), api.got.GetPagination().GetPageSize())
	assert.Equal(t, "100", api.got.GetPagination().GetPageToken())
	assert.Equal(t, []string{"a", "b"}, ids)
	assert.Equal(t, "200", next)

	ids, next, err = clients.SavedJobs{API: &fakeSavedJobs{resp: &savedjobsv1.ListJobSaversResponse{}}}.ListSavers(context.Background(), "job-1", "")
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.Empty(t, next, "no pagination means the last page")

	_, _, err = clients.SavedJobs{API: &fakeSavedJobs{err: errors.New("unavailable")}}.ListSavers(context.Background(), "job-1", "")
	require.Error(t, err)
}
