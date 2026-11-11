package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	jobsearchv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_search/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	callerID = "11111111-1111-4111-8111-111111111111"
	jobID    = "a0000000-0000-4000-8000-000000000001"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func ctxWithCaller(role string) context.Context {
	md := metadata.Pairs("x-user-id", callerID, "x-user-role", role)
	return metadata.NewIncomingContext(context.Background(), md)
}

func newHandler(t *testing.T, repo *mocks.MockSearchRepository) *grpc.JobSearchHandler {
	t.Helper()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	return grpc.NewJobSearchHandler(app.NewSearch(repo, fixedClock{t: now}))
}

func TestSearchJobs_RequiresCaller(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSearchRepository(ctrl)
	h := newHandler(t, repo)

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", callerID))},
		{"bad uuid", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "nope", "x-user-role", "applicant"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.SearchJobs(tt.ctx, &jobsearchv1.SearchJobsRequest{})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("code = %v, want Unauthenticated (%v)", status.Code(err), err)
			}
		})
	}
}

func TestSearchJobs_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSearchRepository(ctrl)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	job := domain.JobSummary{JobID: jobID, Title: "Backend", Deadline: now.Add(time.Hour)}
	repo.EXPECT().Search(gomock.Any(), gomock.Any()).Return([]domain.JobSummary{job}, 1, nil)

	h := newHandler(t, repo)
	resp, err := h.SearchJobs(ctxWithCaller("applicant"), &jobsearchv1.SearchJobsRequest{Query: "back"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetJobs()) != 1 || resp.GetJobs()[0].GetJobId() != jobID {
		t.Fatalf("unexpected jobs: %+v", resp.GetJobs())
	}
	if resp.GetPagination().GetTotalCount() != 1 {
		t.Fatalf("total = %d, want 1", resp.GetPagination().GetTotalCount())
	}
}

func TestSearchJobs_InvalidFilterIsInvalidArgument(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSearchRepository(ctrl)
	h := newHandler(t, repo)
	_, err := h.SearchJobs(ctxWithCaller("applicant"), &jobsearchv1.SearchJobsRequest{CategoryId: "not-a-uuid"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestSearchJobs_RepoErrorIsInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSearchRepository(ctrl)
	repo.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, 0, errors.New("boom"))
	h := newHandler(t, repo)
	_, err := h.SearchJobs(ctxWithCaller("employer"), &jobsearchv1.SearchJobsRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal (%v)", status.Code(err), err)
	}
}