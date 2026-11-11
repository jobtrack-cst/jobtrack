package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobsearchv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_search/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-search/internal/domain"
)

// JobSearchHandler serves the public JobSearchService through the gateway.
type JobSearchHandler struct {
	jobsearchv1.UnimplementedJobSearchServiceServer
	search *app.Search
}

var _ jobsearchv1.JobSearchServiceServer = (*JobSearchHandler)(nil)

func NewJobSearchHandler(search *app.Search) *JobSearchHandler {
	return &JobSearchHandler{search: search}
}

// SearchJobs returns open jobs. Any authenticated role may call it; the
// caller identity is required (UNAUTHENTICATED otherwise). Filters are
// validated in the domain (INVALID_ARGUMENT on failure).
func (h *JobSearchHandler) SearchJobs(ctx context.Context, req *jobsearchv1.SearchJobsRequest) (*jobsearchv1.SearchJobsResponse, error) {
	if _, err := callerFrom(ctx); err != nil {
		return nil, toStatus(err)
	}
	page := req.GetPagination()
	in := app.RawInput{
		Query:      req.GetQuery(),
		CategoryID: req.GetCategoryId(),
		TagIDs:     req.GetTagIds(),
		Location:   req.GetLocation(),
		PageSize:   int(page.GetPageSize()),
		PageToken:  page.GetPageToken(),
	}
	result, err := h.search.Run(ctx, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &jobsearchv1.SearchJobsResponse{
		Jobs:       jobsToProto(result.Jobs),
		Pagination: pageToProto(result.Total, in.PageSize, in.PageToken),
	}, nil
}

func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

func jobsToProto(jobs []domain.JobSummary) []*jobsearchv1.JobSummary {
	out := make([]*jobsearchv1.JobSummary, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, &jobsearchv1.JobSummary{
			JobId:      j.JobID,
			EmployerId: j.EmployerID,
			Title:      j.Title,
			CategoryId: j.CategoryID,
			TagIds:     j.TagIDs,
			Location:   j.Location,
			Deadline:   timestamppb.New(j.Deadline),
		})
	}
	return out
}

// pageToProto builds a shared pagination response. The token is the offset;
// next_page_token is empty when the page reaches the total.
func pageToProto(total, pageSize int, pageToken string) *commonv1.PaginationResponse {
	offset := 0
	if pageToken != "" {
		for _, r := range pageToken {
			if r < '0' || r > '9' {
				offset = 0
				break
			}
			offset = offset*10 + int(r-'0')
		}
	}
	if pageSize <= 0 {
		pageSize = domain.DefaultPageSize
	}
	resp := &commonv1.PaginationResponse{TotalCount: int64(total)}
	if offset+pageSize < total {
		resp.NextPageToken = itoa(offset + pageSize)
	}
	return resp
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidQuery),
		errors.Is(err, domain.ErrInvalidCategoryID),
		errors.Is(err, domain.ErrInvalidTagID),
		errors.Is(err, domain.ErrInvalidLocation),
		errors.Is(err, domain.ErrInvalidPageSize),
		errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}