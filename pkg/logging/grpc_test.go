package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type privateError struct{}

func (privateError) Error() string { panic("untrusted Error must not be called") }

func TestGRPCError_InternalLogsOneSafeJSONEntry(t *testing.T) {
	const method = "/jobtrack.job_posting.v1.JobPostingService/PublishJob"
	const requestID = "11111111-1111-4111-8111-111111111111"
	const secret = "password=hunter2 token=private-token body=private-body alice@example.test"
	for _, tc := range []struct {
		name  string
		cause error
		want  string
	}{
		{"employer verification equivalent", logging.InternalError("employer verification failed: GetCompanyProfile", status.Error(codes.Unavailable, secret)), "GetCompanyProfile: *status.Error (code=Unavailable)"},
		{"wrapped database failure", logging.InternalError("store application", fmt.Errorf("%s: %w", secret, errors.New(secret))), "store application: *fmt.wrapError (code=Unknown): *errors.errorString (code=Unknown)"},
		{"untrusted error renderer", privateError{}, "logging_test.privateError (code=Unknown)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", requestID, "authorization", secret, "x-internal-token", secret, "x-user-id", secret))
			mapped, detailErr := status.New(codes.Internal, "internal error").WithDetails(&errdetails.DebugInfo{Detail: secret})
			require.NoError(t, detailErr)
			err := logging.GRPCError(ctx, logging.New("job-posting", &output), method, tc.cause, mapped.Err())
			require.Equal(t, codes.Internal, status.Code(err))
			require.Equal(t, "internal error", status.Convert(err).Message())
			require.Empty(t, status.Convert(err).Details())
			require.Nil(t, errors.Unwrap(err))
			decoder := json.NewDecoder(&output)
			var entry map[string]any
			require.NoError(t, decoder.Decode(&entry))
			require.Equal(t, io.EOF, decoder.Decode(&map[string]any{}))
			require.Equal(t, "job-posting", entry["service"])
			require.Equal(t, requestID, entry["request_id"])
			require.Equal(t, method, entry["grpc_method"])
			require.Equal(t, "Internal", entry["code"])
			require.Equal(t, "ERROR", entry["level"])
			require.Contains(t, entry["cause"], tc.want)
			raw, e := json.Marshal(entry)
			require.NoError(t, e)
			for _, forbidden := range []string{secret, "hunter2", "private-token", "private-body", "alice@example.test", "internal error", "authorization", "x-internal-token", "x-user-id"} {
				require.NotContains(t, string(raw), forbidden)
				if forbidden != "internal error" {
					require.NotContains(t, err.Error(), forbidden, "client status")
				}
			}
		})
	}
}

func TestInternalError_RetainsCauseWithoutRenderingIt(t *testing.T) {
	cause := &privateError{}
	err := logging.InternalError("employer verification failed", cause)
	require.ErrorIs(t, err, cause)
	require.Equal(t, "employer verification failed", err.Error())
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "employer verification failed", status.Convert(err).Message())
}

func TestGRPCError_ExpectedStatusesDoNotLog(t *testing.T) {
	for _, code := range []codes.Code{codes.OK, codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.Unauthenticated, codes.PermissionDenied, codes.FailedPrecondition, codes.Unavailable, codes.Canceled, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			var output bytes.Buffer
			mapped := status.Error(code, "fixed message")
			require.Equal(t, mapped, logging.GRPCError(context.Background(), logging.New("offer", &output), "/method", privateError{}, mapped))
			require.Empty(t, output.String())
		})
	}
}

func TestGRPCError_RejectsUntrustedRequestIDs(t *testing.T) {
	for _, ids := range [][]string{nil, {"password=private-token"}, {strings.Repeat("a", 10000)}, {"00000000-0000-0000-0000-000000000000"}, {"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}} {
		var output bytes.Buffer
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"x-request-id": ids})
		logging.GRPCError(ctx, logging.New("offer", &output), "/method", privateError{}, status.Error(codes.Internal, "internal error"))
		var entry map[string]any
		require.NoError(t, json.Unmarshal(output.Bytes(), &entry))
		require.NotContains(t, entry, "request_id")
	}
}
