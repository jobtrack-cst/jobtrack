package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	jpv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/clients"
	adapter "github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const loggingSecret = "password=hidden token=private body=private user@example.test"
const loggingRequestID = "aaaaaaaa-1111-4111-8111-111111111111"

func loggingContext(ids []string) context.Context {
	md := metadata.Pairs("x-user-id", ownerID, "x-user-role", "employer", "authorization", loggingSecret, "x-internal-token", loggingSecret)
	md["x-request-id"] = ids
	return metadata.NewIncomingContext(context.Background(), md)
}

func jobLog(t *testing.T, output *bytes.Buffer, method, requestID string) map[string]any {
	t.Helper()
	raw := output.String()
	for _, forbidden := range []string{loggingSecret, "hidden", "private", "user@example.test", ownerID, jobID, "authorization", "x-internal-token"} {
		require.NotContains(t, raw, forbidden)
	}
	decoder := json.NewDecoder(output)
	var entry map[string]any
	require.NoError(t, decoder.Decode(&entry))
	require.Equal(t, io.EOF, decoder.Decode(&map[string]any{}), "one log entry per failed RPC")
	require.Equal(t, "job-posting", entry["service"])
	require.Equal(t, method, entry["grpc_method"])
	require.Equal(t, "Internal", entry["code"])
	require.Equal(t, "ERROR", entry["level"])
	if requestID == "" {
		require.NotContains(t, entry, "request_id")
	} else {
		require.Equal(t, requestID, entry["request_id"])
	}
	return entry
}

// Exercise the real employer adapter and app, with the generated RPC client
// boundary stubbed: downstream text/details must be stripped before logging.
type failingEmployerRPC struct {
	employerv1.EmployerServiceClient
	failure      error
	verification employerv1.VerificationStatus
}

func (c failingEmployerRPC) GetCompanyProfile(context.Context, *employerv1.GetCompanyProfileRequest, ...grpc.CallOption) (*employerv1.GetCompanyProfileResponse, error) {
	return &employerv1.GetCompanyProfileResponse{Profile: &employerv1.CompanyProfile{CompanyName: loggingSecret, VerificationStatus: c.verification}}, c.failure
}

func TestJobPosting_PublishEmployerFailureLogsOneSafeWrappedCause(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		failure               error
		verification          employerv1.VerificationStatus
		ids                   []string
		diagnostic, requestID string
	}{
		{name: "unavailable", failure: status.Error(codes.Unavailable, loggingSecret), ids: []string{loggingRequestID}, diagnostic: "code=Unavailable", requestID: loggingRequestID},
		{name: "internal", failure: status.Error(codes.Internal, loggingSecret), diagnostic: "code=Internal"},
		{name: "deadline remains internal", failure: status.Error(codes.DeadlineExceeded, loggingSecret), diagnostic: "code=DeadlineExceeded"},
		{name: "untrusted request ID", failure: status.Error(codes.Unavailable, loggingSecret), ids: []string{loggingSecret}, diagnostic: "code=Unavailable"},
		{name: "duplicate request ID", failure: status.Error(codes.Unavailable, loggingSecret), ids: []string{loggingRequestID, loggingRequestID}, diagnostic: "code=Unavailable"},
		{name: "unexpected verification", verification: employerv1.VerificationStatus(99), diagnostic: "GetCompanyProfile returned unexpected verification status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockJobRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			clock.EXPECT().Now().Return(now)
			draft := storedJob(domain.StatusDraft)
			draft.Description = loggingSecret
			repo.EXPECT().GetByID(gomock.Any(), jobID).Return(draft, nil)
			verifier := clients.NewEmployerClient(failingEmployerRPC{failure: tc.failure, verification: tc.verification})
			// Verify the sentinel survives the controlled wrappers without copying the
			// remote message. No logging occurs at this client boundary.
			_, cause := verifier.IsVerified(context.Background(), ownerID)
			require.ErrorIs(t, cause, domain.ErrEmployerCheckFailed)
			require.NotContains(t, cause.Error(), loggingSecret)
			var output bytes.Buffer
			h := adapter.NewJobPostingHandler(app.NewJobs(repo, verifier, clock), logging.New("job-posting", &output))
			resp, err := h.PublishJob(loggingContext(tc.ids), &jpv1.PublishJobRequest{JobId: jobID})
			require.Nil(t, resp)
			require.Equal(t, codes.Internal, status.Code(err))
			require.Equal(t, domain.ErrEmployerCheckFailed.Error(), status.Convert(err).Message())
			require.Empty(t, status.Convert(err).Details())
			require.Nil(t, errors.Unwrap(err))
			require.NotContains(t, err.Error(), "GetCompanyProfile")
			require.NotContains(t, err.Error(), loggingSecret)
			entry := jobLog(t, &output, jpv1.JobPostingService_PublishJob_FullMethodName, tc.requestID)
			require.Contains(t, entry["cause"], "GetCompanyProfile")
			require.Contains(t, entry["cause"], tc.diagnostic)
			require.Contains(t, entry["cause"], "*errors.errorString", "domain sentinel retained in chain")
		})
	}
}

func TestJobPosting_UnexpectedErrorsLogOnceForEveryRPC(t *testing.T) {
	for _, method := range []string{"CreateJob", "UpdateJob", "PublishJob", "CloseJob", "GetJob", "ListMyJobs", "ListJobsClosingBetween"} {
		t.Run(method, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockJobRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			clock.EXPECT().Now().Return(now).AnyTimes()
			failure := fmt.Errorf("%s: %w", loggingSecret, errors.New(loggingSecret))
			switch method {
			case "CreateJob":
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.Job{}, failure)
			case "ListMyJobs":
				repo.EXPECT().ListByEmployer(gomock.Any(), ownerID, gomock.Any(), gomock.Any()).Return(nil, 0, failure)
			case "ListJobsClosingBetween":
				repo.EXPECT().ListClosingBetween(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, 0, failure)
			default:
				repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, failure)
			}
			var output bytes.Buffer
			logger := logging.New("job-posting", &output)
			svc := app.NewJobs(repo, nil, clock)
			h := adapter.NewJobPostingHandler(svc, logger)
			internal := adapter.NewJobPostingInternalHandler(svc, logger)
			ctx := loggingContext([]string{loggingRequestID})
			var err error
			var fullMethod string
			switch method {
			case "CreateJob":
				req := createRequest()
				req.Description = loggingSecret
				_, err = h.CreateJob(ctx, req)
				fullMethod = jpv1.JobPostingService_CreateJob_FullMethodName
			case "UpdateJob":
				req := createRequest()
				_, err = h.UpdateJob(ctx, &jpv1.UpdateJobRequest{JobId: jobID, Title: req.Title, Description: loggingSecret, Deadline: req.Deadline})
				fullMethod = jpv1.JobPostingService_UpdateJob_FullMethodName
			case "PublishJob":
				_, err = h.PublishJob(ctx, &jpv1.PublishJobRequest{JobId: jobID})
				fullMethod = jpv1.JobPostingService_PublishJob_FullMethodName
			case "CloseJob":
				_, err = h.CloseJob(ctx, &jpv1.CloseJobRequest{JobId: jobID})
				fullMethod = jpv1.JobPostingService_CloseJob_FullMethodName
			case "GetJob":
				_, err = h.GetJob(ctx, &jpv1.GetJobRequest{JobId: jobID})
				fullMethod = jpv1.JobPostingService_GetJob_FullMethodName
			case "ListMyJobs":
				_, err = h.ListMyJobs(ctx, &jpv1.ListMyJobsRequest{})
				fullMethod = jpv1.JobPostingService_ListMyJobs_FullMethodName
			case "ListJobsClosingBetween":
				_, err = internal.ListJobsClosingBetween(ctx, &jpv1.ListJobsClosingBetweenRequest{StartTime: timestamppb.New(now), EndTime: timestamppb.New(now.AddDate(0, 0, 1))})
				fullMethod = jpv1.JobPostingInternalService_ListJobsClosingBetween_FullMethodName
			}
			require.Equal(t, "internal error", status.Convert(err).Message())
			require.Empty(t, status.Convert(err).Details())
			entry := jobLog(t, &output, fullMethod, loggingRequestID)
			require.Contains(t, entry["cause"], "*fmt.wrapError")
			require.Contains(t, entry["cause"], "*errors.errorString")
		})
	}
}

func TestJobPosting_ExpectedErrorsDoNotLog(t *testing.T) {
	for _, tc := range []struct {
		failure error
		code    codes.Code
	}{
		{domain.ErrJobNotFound, codes.NotFound}, {domain.ErrNotJobOwner, codes.PermissionDenied},
		{context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockJobRepository(ctrl)
			repo.EXPECT().GetByID(gomock.Any(), jobID).Return(domain.Job{}, tc.failure)
			var output bytes.Buffer
			h := adapter.NewJobPostingHandler(app.NewJobs(repo, nil, nil), logging.New("job-posting", &output))
			_, err := h.GetJob(asOwner(), &jpv1.GetJobRequest{JobId: jobID})
			require.Equal(t, tc.code, status.Code(err))
			require.Empty(t, output.String())
		})
	}
}

func TestJobPosting_HealthUnexpectedErrorsLogOnce(t *testing.T) {
	for _, tc := range []struct {
		state   domain.HealthStatus
		failure error
	}{
		{domain.Unknown, errors.New(loggingSecret)}, {domain.HealthStatus(99), nil},
	} {
		var output bytes.Buffer
		checker := healthCheckerFunc(func(context.Context) (domain.HealthStatus, error) { return tc.state, tc.failure })
		h := adapter.NewHealthHandler(checker, logging.New("job-posting", &output))
		_, err := h.Check(loggingContext([]string{loggingRequestID}), &healthpb.HealthCheckRequest{})
		require.Equal(t, "health check failed", status.Convert(err).Message())
		jobLog(t, &output, healthpb.Health_Check_FullMethodName, loggingRequestID)
	}
}
