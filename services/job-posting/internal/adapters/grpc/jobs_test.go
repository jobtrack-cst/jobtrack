package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jpv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	adapter "github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports/mocks"
)

const (
	ownerID = "11111111-1111-4111-8111-111111111111"
	otherID = "22222222-2222-4222-8222-222222222222"
	jobID   = "33333333-3333-4333-8333-333333333333"
	tagID   = "44444444-4444-4444-8444-444444444444"
)

var (
	errDB = errors.New("pq: secret SQL detail")
	now   = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	bt    = time.FixedZone("BTT", 6*3600)
)

type handlers struct {
	public   *adapter.JobPostingHandler
	internal *adapter.JobPostingInternalHandler
	repo     *mocks.MockJobRepository
	verifier *mocks.MockEmployerVerifier
}

// Handlers are tested with the real app layer and mocked ports.
func newHandlers(t *testing.T) handlers {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	h := handlers{repo: mocks.NewMockJobRepository(ctrl), verifier: mocks.NewMockEmployerVerifier(ctrl)}
	svc := app.NewJobs(h.repo, h.verifier, clock)
	h.public, h.internal = adapter.NewJobPostingHandler(svc), adapter.NewJobPostingInternalHandler(svc)
	return h
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asOwner() context.Context { return withCaller(ownerID, "employer") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func storedJob(st domain.Status) domain.Job {
	return domain.Job{
		ID: jobID, EmployerID: ownerID, Status: st,
		JobFields: domain.JobFields{
			Title: "Junior Go Developer", Description: "Build services.", CategoryID: tagID,
			TagIDs: []string{tagID}, Location: "Thimphu", Deadline: now.Add(48 * time.Hour),
		},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
}

func createRequest() *jpv1.CreateJobRequest {
	return &jpv1.CreateJobRequest{
		Title: "Junior Go Developer", Description: "Build services.", CategoryId: tagID,
		TagIds: []string{tagID}, Location: "Thimphu", Deadline: timestamppb.New(now.Add(48 * time.Hour).In(bt)),
	}
}

func TestJobPostingHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", ownerID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "employer"))},
		{"user ID not a UUID", withCaller("abc", "employer")},
		{"unknown role", withCaller(ownerID, "superuser")},
		{"uppercase role", withCaller(ownerID, "Employer")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlers(t) // no repository calls expected
			_, err := h.public.CreateJob(tt.ctx, createRequest())
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.public.GetJob(tt.ctx, &jpv1.GetJobRequest{JobId: jobID})
			wantCode(t, err, codes.Unauthenticated)
			_, err = h.public.ListMyJobs(tt.ctx, &jpv1.ListMyJobsRequest{})
			wantCode(t, err, codes.Unauthenticated)
		})
	}
}

// Role matrix for the employer-only RPCs. Only the role is wrong, so a
// non-employer is denied before any repository call.
func TestJobPostingHandler_EmployerOnlyRPCs(t *testing.T) {
	calls := map[string]func(adapter.JobPostingHandler, context.Context) error{
		"CreateJob": func(h adapter.JobPostingHandler, c context.Context) error {
			_, err := h.CreateJob(c, createRequest())
			return err
		},
		"UpdateJob": func(h adapter.JobPostingHandler, c context.Context) error {
			_, err := h.UpdateJob(c, &jpv1.UpdateJobRequest{JobId: jobID, Title: "T", Deadline: timestamppb.New(now.Add(time.Hour))})
			return err
		},
		"PublishJob": func(h adapter.JobPostingHandler, c context.Context) error {
			_, err := h.PublishJob(c, &jpv1.PublishJobRequest{JobId: jobID})
			return err
		},
		"CloseJob": func(h adapter.JobPostingHandler, c context.Context) error {
			_, err := h.CloseJob(c, &jpv1.CloseJobRequest{JobId: jobID})
			return err
		},
		"ListMyJobs": func(h adapter.JobPostingHandler, c context.Context) error {
			_, err := h.ListMyJobs(c, &jpv1.ListMyJobsRequest{})
			return err
		},
	}
	for name, call := range calls {
		for _, role := range []string{"applicant", "admin"} {
			t.Run(name+" as "+role, func(t *testing.T) {
				h := newHandlers(t)
				wantCode(t, call(*h.public, withCaller(ownerID, role)), codes.PermissionDenied)
			})
		}
	}
}

// The role is checked before the input, so non-employers learn nothing else.
func TestJobPostingHandler_RoleBeforeInput(t *testing.T) {
	badDeadline := &timestamppb.Timestamp{Seconds: 1, Nanos: -1}
	h := newHandlers(t)
	_, err := h.public.CreateJob(withCaller(otherID, "applicant"), &jpv1.CreateJobRequest{Deadline: badDeadline})
	wantCode(t, err, codes.PermissionDenied)
	_, err = h.public.UpdateJob(withCaller(otherID, "admin"), &jpv1.UpdateJobRequest{JobId: "bad", Deadline: badDeadline})
	wantCode(t, err, codes.PermissionDenied)
	_, err = h.public.ListMyJobs(withCaller(otherID, "applicant"), &jpv1.ListMyJobsRequest{Status: jpv1.JobStatus(99)})
	wantCode(t, err, codes.PermissionDenied)
}

func TestJobPostingHandler_CreateJob_MapsRequestAndResponse(t *testing.T) {
	h := newHandlers(t)
	h.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, j domain.Job) (domain.Job, error) {
		assert.Equal(t, ownerID, j.EmployerID)
		assert.Equal(t, "Junior Go Developer", j.Title)
		assert.Equal(t, "Build services.", j.Description)
		assert.Equal(t, tagID, j.CategoryID)
		assert.Equal(t, []string{tagID}, j.TagIDs)
		assert.Equal(t, "Thimphu", j.Location)
		assert.Equal(t, now.Add(48*time.Hour), j.Deadline, "deadline converted to UTC")
		assert.Equal(t, domain.StatusDraft, j.Status)
		j.ID = jobID
		return j, nil
	})

	resp, err := h.public.CreateJob(asOwner(), createRequest())

	require.NoError(t, err)
	got := resp.GetJob()
	assert.Equal(t, jobID, got.GetJobId())
	assert.Equal(t, ownerID, got.GetEmployerId())
	assert.Equal(t, []string{tagID}, got.GetTagIds())
	assert.Equal(t, jpv1.JobStatus_JOB_STATUS_DRAFT, got.GetStatus())
	assert.True(t, got.GetDeadline().AsTime().Equal(now.Add(48*time.Hour)))
	assert.True(t, got.GetTimestamps().GetCreatedAt().AsTime().Equal(now))
	assert.True(t, got.GetTimestamps().GetUpdatedAt().AsTime().Equal(now))
}

func TestJobPostingHandler_InvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		req  *jpv1.CreateJobRequest
	}{
		{"nil request", nil},
		{"missing deadline", &jpv1.CreateJobRequest{Title: "T"}},
		{"deadline with negative nanos", &jpv1.CreateJobRequest{Title: "T", Deadline: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}}},
		{"deadline after year 9999", &jpv1.CreateJobRequest{Title: "T", Deadline: &timestamppb.Timestamp{Seconds: 253402300800}}},
		{"deadline in the past", &jpv1.CreateJobRequest{Title: "T", Deadline: timestamppb.New(now.Add(-time.Second))}},
		{"empty title", &jpv1.CreateJobRequest{Deadline: timestamppb.New(now.Add(time.Hour))}},
		{"title 151 chars", &jpv1.CreateJobRequest{Title: strings.Repeat("a", 151), Deadline: timestamppb.New(now.Add(time.Hour))}},
		{"tag not a UUID", &jpv1.CreateJobRequest{Title: "T", TagIds: []string{"go"}, Deadline: timestamppb.New(now.Add(time.Hour))}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newHandlers(t).public.CreateJob(asOwner(), tt.req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

// Every lifecycle rule reaches the client as the Section 9 code.
func TestJobPostingHandler_LifecycleCodes(t *testing.T) {
	future := timestamppb.New(now.Add(time.Hour))
	update := func(h handlers, c context.Context) error {
		_, err := h.public.UpdateJob(c, &jpv1.UpdateJobRequest{JobId: jobID, Title: "New", Deadline: future})
		return err
	}
	publish := func(h handlers, c context.Context) error {
		_, err := h.public.PublishJob(c, &jpv1.PublishJobRequest{JobId: jobID})
		return err
	}
	closeJob := func(h handlers, c context.Context) error {
		_, err := h.public.CloseJob(c, &jpv1.CloseJobRequest{JobId: jobID})
		return err
	}
	get := func(st domain.Status) func(h handlers) {
		return func(h handlers) { h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(st), nil) }
	}
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		call  func(handlers, context.Context) error
		setup func(handlers)
		want  codes.Code
	}{
		{"update closed job", asOwner(), update, get(domain.StatusClosed), codes.FailedPrecondition},
		{"update removed job", asOwner(), update, get(domain.StatusRemoved), codes.FailedPrecondition},
		{"update by other employer", withCaller(otherID, "employer"), update, get(domain.StatusDraft), codes.PermissionDenied},
		{"update unknown job", asOwner(), update, func(h handlers) {
			h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, domain.ErrJobNotFound)
		}, codes.NotFound},
		{"update job id not a UUID", asOwner(), func(h handlers, c context.Context) error {
			_, err := h.public.UpdateJob(c, &jpv1.UpdateJobRequest{JobId: "x", Title: "New", Deadline: future})
			return err
		}, nil, codes.InvalidArgument},
		{"publish published job", asOwner(), publish, get(domain.StatusPublished), codes.FailedPrecondition},
		{"publish not verified", asOwner(), publish, func(h handlers) {
			get(domain.StatusDraft)(h)
			h.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(false, nil)
		}, codes.FailedPrecondition},
		{"publish verified", asOwner(), publish, func(h handlers) {
			get(domain.StatusDraft)(h)
			h.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(true, nil)
			h.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusDraft, domain.StatusPublished, now).
				Return(storedJob(domain.StatusPublished), nil)
		}, codes.OK},
		{"publish race lost", asOwner(), publish, func(h handlers) {
			get(domain.StatusDraft)(h)
			h.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(true, nil)
			h.repo.EXPECT().UpdateStatus(gomock.Any(), jobID, domain.StatusDraft, domain.StatusPublished, now).
				Return(domain.Job{}, domain.ErrJobStatusChanged)
		}, codes.FailedPrecondition},
		{"close draft job", asOwner(), closeJob, get(domain.StatusDraft), codes.FailedPrecondition},
		{"close by admin", withCaller(otherID, "admin"), closeJob, nil, codes.PermissionDenied},
		{"close database error", asOwner(), closeJob, func(h handlers) {
			h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, errDB)
		}, codes.Internal},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlers(t)
			if tt.setup != nil {
				tt.setup(h)
			}
			wantCode(t, tt.call(h, tt.ctx), tt.want)
		})
	}
}

func TestJobPostingHandler_InternalErrorsDoNotLeak(t *testing.T) {
	h := newHandlers(t)
	h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, errDB)
	_, err := h.public.GetJob(asOwner(), &jpv1.GetJobRequest{JobId: jobID})
	wantCode(t, err, codes.Internal)
	assert.NotContains(t, status.Convert(err).Message(), "secret")
}

func TestJobPostingHandler_GetJob(t *testing.T) {
	for _, tt := range []struct {
		name   string
		ctx    context.Context
		status domain.Status
		want   codes.Code
	}{
		{"applicant sees published", withCaller(otherID, "applicant"), domain.StatusPublished, codes.OK},
		{"applicant cannot see draft", withCaller(otherID, "applicant"), domain.StatusDraft, codes.NotFound},
		{"other employer cannot see removed", withCaller(otherID, "employer"), domain.StatusRemoved, codes.NotFound},
		{"admin sees removed", withCaller(otherID, "admin"), domain.StatusRemoved, codes.OK},
		{"owner sees draft", asOwner(), domain.StatusDraft, codes.OK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlers(t)
			h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(tt.status), nil)
			resp, err := h.public.GetJob(tt.ctx, &jpv1.GetJobRequest{JobId: jobID})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK {
				assert.Equal(t, jobID, resp.GetJob().GetJobId())
				assert.True(t, resp.GetJob().GetDeadline().AsTime().Equal(now.Add(48*time.Hour)), "status and deadline for the open check")
			}
		})
	}
	t.Run("job id not a UUID", func(t *testing.T) {
		_, err := newHandlers(t).public.GetJob(asOwner(), &jpv1.GetJobRequest{JobId: "nope"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestJobPostingHandler_ListMyJobs(t *testing.T) {
	filters := map[jpv1.JobStatus]domain.Status{
		jpv1.JobStatus_JOB_STATUS_UNSPECIFIED: domain.StatusUnspecified,
		jpv1.JobStatus_JOB_STATUS_DRAFT:       domain.StatusDraft,
		jpv1.JobStatus_JOB_STATUS_PUBLISHED:   domain.StatusPublished,
		jpv1.JobStatus_JOB_STATUS_CLOSED:      domain.StatusClosed,
		jpv1.JobStatus_JOB_STATUS_REMOVED:     domain.StatusRemoved,
	}
	for protoStatus, want := range filters {
		t.Run("filter "+protoStatus.String(), func(t *testing.T) {
			h := newHandlers(t)
			h.repo.EXPECT().ListByEmployer(gomock.Any(), ownerID, want, domain.Page{Limit: 2, Offset: 2}).
				Return([]domain.Job{storedJob(domain.StatusPublished)}, 5, nil)
			resp, err := h.public.ListMyJobs(asOwner(), &jpv1.ListMyJobsRequest{
				Status: protoStatus, Pagination: &commonv1.PaginationRequest{PageSize: 2, PageToken: "2"},
			})
			require.NoError(t, err)
			assert.Len(t, resp.GetJobs(), 1)
			assert.Equal(t, int64(5), resp.GetPagination().GetTotalCount())
			assert.Equal(t, "4", resp.GetPagination().GetNextPageToken())
		})
	}
	for _, tt := range []struct {
		name string
		req  *jpv1.ListMyJobsRequest
	}{
		{"unknown status", &jpv1.ListMyJobsRequest{Status: jpv1.JobStatus(99)}},
		{"negative page size", &jpv1.ListMyJobsRequest{Pagination: &commonv1.PaginationRequest{PageSize: -1}}},
		{"bad page token", &jpv1.ListMyJobsRequest{Pagination: &commonv1.PaginationRequest{PageToken: "abc"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newHandlers(t).public.ListMyJobs(asOwner(), tt.req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestJobPostingInternalHandler_ListJobsClosingBetween(t *testing.T) {
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, bt)
	end := start.Add(48 * time.Hour)
	t.Run("lists the window in UTC with no user identity", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().ListClosingBetween(gomock.Any(), domain.ClosingWindow{Start: start.UTC(), End: end.UTC()}, domain.Page{Limit: 20}).
			Return([]domain.Job{storedJob(domain.StatusPublished)}, 1, nil)
		resp, err := h.internal.ListJobsClosingBetween(context.Background(), &jpv1.ListJobsClosingBetweenRequest{
			StartTime: timestamppb.New(start), EndTime: timestamppb.New(end),
		})
		require.NoError(t, err)
		require.Len(t, resp.GetJobs(), 1)
		assert.Equal(t, jpv1.JobStatus_JOB_STATUS_PUBLISHED, resp.GetJobs()[0].GetStatus())
		assert.Equal(t, int64(1), resp.GetPagination().GetTotalCount())
		assert.Equal(t, "", resp.GetPagination().GetNextPageToken())
	})
	for _, tt := range []struct {
		name       string
		start, end *timestamppb.Timestamp
	}{
		{"no times", nil, nil},
		{"missing start", nil, timestamppb.New(end)},
		{"missing end", timestamppb.New(start), nil},
		{"start equal to end", timestamppb.New(start), timestamppb.New(start)},
		{"start after end", timestamppb.New(end), timestamppb.New(start)},
		{"start with negative nanos", &timestamppb.Timestamp{Seconds: 1, Nanos: -1}, timestamppb.New(end)},
		{"end after year 9999", timestamppb.New(start), &timestamppb.Timestamp{Seconds: 253402300800}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newHandlers(t).internal.ListJobsClosingBetween(context.Background(),
				&jpv1.ListJobsClosingBetweenRequest{StartTime: tt.start, EndTime: tt.end})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
	t.Run("database error", func(t *testing.T) {
		h := newHandlers(t)
		h.repo.EXPECT().ListClosingBetween(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, 0, errDB)
		_, err := h.internal.ListJobsClosingBetween(context.Background(), &jpv1.ListJobsClosingBetweenRequest{
			StartTime: timestamppb.New(start), EndTime: timestamppb.New(end),
		})
		wantCode(t, err, codes.Internal)
	})
}

func TestJobPostingHandler_PublishEmployerCheckFailedIsInternalWithFixedMessage(t *testing.T) {
	h := newHandlers(t)
	h.repo.EXPECT().GetByID(gomock.Any(), jobID).Return(storedJob(domain.StatusDraft), nil)
	// The employer client wraps downstream details; they must not reach the caller.
	h.verifier.EXPECT().IsVerified(gomock.Any(), ownerID).Return(false, errors.Join(domain.ErrEmployerCheckFailed, errDB))
	_, err := h.public.PublishJob(asOwner(), &jpv1.PublishJobRequest{JobId: jobID})
	wantCode(t, err, codes.Internal)
	assert.Equal(t, domain.ErrEmployerCheckFailed.Error(), status.Convert(err).Message())
}
