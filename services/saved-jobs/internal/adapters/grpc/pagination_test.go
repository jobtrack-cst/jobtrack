package grpc

import (
	"context"
	"testing"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/saved-jobs/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRepo struct {
	savers     []string
	lastLimit  int
	lastOffset int
}

func (f *fakeRepo) SaveJob(ctx context.Context, jobID, applicantID string) error   { return nil }
func (f *fakeRepo) UnsaveJob(ctx context.Context, jobID, applicantID string) error { return nil }
func (f *fakeRepo) ListMySavedJobs(ctx context.Context, applicantID string, limit, offset int) (domain.Page, error) {
	return domain.Page{}, nil
}
func (f *fakeRepo) ListJobSavers(ctx context.Context, jobID string, limit, offset int) ([]string, int, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	total := len(f.savers)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return f.savers[start:end], total, nil
}

func makeHandler(savers []string) *SavedJobsInternalHandler {
	repo := &fakeRepo{savers: savers}
	svc := app.NewSavedJobs(repo)
	return NewSavedJobsInternalHandler(svc)
}

func TestListJobSavers_PageSizeDefaultsTo20(t *testing.T) {
	h := makeHandler(makeSavers(50))
	resp, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId: "33333333-3333-4333-8333-333333333333",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ApplicantIds) != 20 {
		t.Errorf("expected 20 savers, got %d", len(resp.ApplicantIds))
	}
	if resp.Pagination.GetTotalCount() != 50 {
		t.Errorf("expected total 50, got %d", resp.Pagination.GetTotalCount())
	}
	if resp.Pagination.GetNextPageToken() != "20" {
		t.Errorf("expected next_page_token '20', got %q", resp.Pagination.GetNextPageToken())
	}
}

func TestListJobSavers_NegativePageSize(t *testing.T) {
	h := makeHandler(makeSavers(10))
	_, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId:      "33333333-3333-4333-8333-333333333333",
		Pagination: &commonv1.PaginationRequest{PageSize: -1},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestListJobSavers_CapsAt100(t *testing.T) {
	h := makeHandler(makeSavers(200))
	resp, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId:      "33333333-3333-4333-8333-333333333333",
		Pagination: &commonv1.PaginationRequest{PageSize: 500},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ApplicantIds) != 100 {
		t.Errorf("expected 100 savers, got %d", len(resp.ApplicantIds))
	}
}

func TestListJobSavers_ValidPageToken(t *testing.T) {
	h := makeHandler(makeSavers(30))
	resp, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId:      "33333333-3333-4333-8333-333333333333",
		Pagination: &commonv1.PaginationRequest{PageSize: 10, PageToken: "10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ApplicantIds) != 10 {
		t.Errorf("expected 10 savers, got %d", len(resp.ApplicantIds))
	}
	if resp.Pagination.GetNextPageToken() != "20" {
		t.Errorf("expected next_page_token '20', got %q", resp.Pagination.GetNextPageToken())
	}
}

func TestListJobSavers_InvalidPageToken(t *testing.T) {
	h := makeHandler(makeSavers(10))
	_, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId:      "33333333-3333-4333-8333-333333333333",
		Pagination: &commonv1.PaginationRequest{PageToken: "abc"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestListJobSavers_EmptyNextTokenOnLastPage(t *testing.T) {
	h := makeHandler(makeSavers(15))
	resp, err := h.ListJobSavers(context.Background(), &savedjobsv1.ListJobSaversRequest{
		JobId:      "33333333-3333-4333-8333-333333333333",
		Pagination: &commonv1.PaginationRequest{PageSize: 10, PageToken: "10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Pagination.GetNextPageToken() != "" {
		t.Errorf("expected empty next_page_token on last page, got %q", resp.Pagination.GetNextPageToken())
	}
}

func makeSavers(n int) []string {
	savers := make([]string, n)
	for i := 0; i < n; i++ {
		savers[i] = "user-" + string(rune('a'+i%26))
	}
	return savers
}
