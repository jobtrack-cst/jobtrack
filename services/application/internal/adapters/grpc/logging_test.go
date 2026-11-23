package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	pb "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	adapter "github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const logID = "11111111-1111-4111-8111-111111111111"
const secret = "password=hidden token=private body=private user@example.test"

func TestUnexpectedErrors_AllRPCsLogExactlyOnce(t *testing.T) {
	for _, method := range []string{"SubmitApplication", "WithdrawApplication", "GetApplication", "ListApplications"} {
		t.Run(method, func(t *testing.T) {
			const id = logID
			ctrl := gomock.NewController(t)
			failure := fmt.Errorf("private wrapper %s: %w", secret, errors.New(secret))
			repo := mocks.NewMockRepository(ctrl)

			switch method {
			case "SubmitApplication":
				repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), failure)
			case "WithdrawApplication", "GetApplication":
				repo.EXPECT().Get(gomock.Any(), id).Return(domain.Application{}, failure)
			case "ListApplications":
				repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), failure)
			}
			var output bytes.Buffer
			logger := logging.New("application", &output)
			h := adapter.NewApplicationHandler(app.NewService(repo, nil, nil, nil), logger)
			role := "applicant"
			if method == "MakeOffer" {
				role = "employer"
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role, "x-request-id", id, "authorization", secret, "x-internal-token", secret))
			var err error
			var fullMethod string
			switch method {
			case "SubmitApplication":
				_, err = h.SubmitApplication(ctx, &pb.SubmitApplicationRequest{JobId: id, ResumeId: id})
				fullMethod = pb.ApplicationService_SubmitApplication_FullMethodName
			case "WithdrawApplication":
				_, err = h.WithdrawApplication(ctx, &pb.WithdrawApplicationRequest{ApplicationId: id})
				fullMethod = pb.ApplicationService_WithdrawApplication_FullMethodName
			case "GetApplication":
				_, err = h.GetApplication(ctx, &pb.GetApplicationRequest{ApplicationId: id})
				fullMethod = pb.ApplicationService_GetApplication_FullMethodName
			case "ListApplications":
				_, err = h.ListApplications(ctx, &pb.ListApplicationsRequest{})
				fullMethod = pb.ApplicationService_ListApplications_FullMethodName
			}
			require.Equal(t, codes.Internal, status.Code(err))
			require.NotContains(t, err.Error(), "private")
			require.NotContains(t, err.Error(), "code=Unknown")
			require.Empty(t, status.Convert(err).Details())
			entry := assertLog(t, &output, fullMethod)
			require.Contains(t, entry["cause"], "*fmt.wrapError")
			require.Contains(t, entry["cause"], "*errors.errorString")
			if method == "GetApplication" {
				require.Contains(t, entry["cause"], "application storage failed")
			}
		})
	}
}
func assertLog(t *testing.T, output *bytes.Buffer, method string) map[string]any {
	t.Helper()
	raw := output.String()
	for _, private := range []string{secret, "hidden", "private", "user@example.test", "authorization", "x-internal-token", "internal error", "subscription operation failed"} {
		require.NotContains(t, raw, private)
	}
	decoder := json.NewDecoder(output)
	var entry map[string]any
	require.NoError(t, decoder.Decode(&entry))
	require.Equal(t, io.EOF, decoder.Decode(&map[string]any{}))
	require.Equal(t, "application", entry["service"])
	require.Equal(t, method, entry["grpc_method"])
	require.Equal(t, "Internal", entry["code"])
	require.Equal(t, "ERROR", entry["level"])
	require.Equal(t, logID, entry["request_id"])
	return entry
}

type loggingHealth struct {
	err   error
	state domain.HealthStatus
}

func (h loggingHealth) CheckHealth(context.Context) (domain.HealthStatus, error) {
	return h.state, h.err
}
func TestUnexpectedErrors_HealthLogsExactlyOnce(t *testing.T) {
	for _, checker := range []loggingHealth{{err: errors.New(secret)}, {state: domain.HealthStatus(99)}} {
		var output bytes.Buffer
		h := adapter.NewHealthHandler(checker, logging.New("application", &output))
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", logID))
		_, err := h.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Equal(t, codes.Internal, status.Code(err))
		require.Equal(t, "health check failed", status.Convert(err).Message())
		assertLog(t, &output, healthpb.Health_Check_FullMethodName)
	}
}

func TestUnexpectedErrors_EligibilityFailureRetainsSafeCause(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockRepository(ctrl)
	jobs := mocks.NewMockJobPosting(ctrl)
	resumes := mocks.NewMockResume(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
	jobs.EXPECT().GetJob(gomock.Any(), logID).Return(domain.Job{}, status.Error(codes.Internal, secret))
	var output bytes.Buffer
	h := adapter.NewApplicationHandler(app.NewService(repo, jobs, resumes, mocks.NewMockUsers(ctrl)), logging.New("application", &output))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", logID, "x-user-role", "applicant", "x-request-id", logID))
	_, err := h.SubmitApplication(ctx, &pb.SubmitApplicationRequest{JobId: logID, ResumeId: logID})
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "eligibility dependency failed", status.Convert(err).Message())
	entry := assertLog(t, &output, pb.ApplicationService_SubmitApplication_FullMethodName)
	require.Contains(t, entry["cause"], "eligibility dependency failed: *status.Error (code=Internal)")
	require.Nil(t, errors.Unwrap(err))
}
