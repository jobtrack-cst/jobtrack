package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	pb "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	adapter "github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports/mocks"
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
	for _, method := range []string{"SendMessage", "ListMessages"} {
		t.Run(method, func(t *testing.T) {
			const id = logID
			ctrl := gomock.NewController(t)
			failure := fmt.Errorf("private wrapper %s: %w", secret, errors.New(secret))
			participants := mocks.NewMockParticipantRepository(ctrl)
			participants.EXPECT().GetByApplicationID(gomock.Any(), id).Return(domain.ApplicationParticipants{}, failure)
			var output bytes.Buffer
			logger := logging.New("messaging", &output)
			h := adapter.NewMessageHandler(app.NewMessages(nil, participants, nil), logger)
			role := "applicant"
			if method == "MakeOffer" {
				role = "employer"
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role, "x-request-id", id, "authorization", secret, "x-internal-token", secret))
			var err error
			var fullMethod string
			switch method {
			case "SendMessage":
				_, err = h.SendMessage(ctx, &pb.SendMessageRequest{ApplicationId: id, Content: secret})
				fullMethod = pb.MessagingService_SendMessage_FullMethodName
			case "ListMessages":
				_, err = h.ListMessages(ctx, &pb.ListMessagesRequest{ApplicationId: id})
				fullMethod = pb.MessagingService_ListMessages_FullMethodName
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
	require.Equal(t, "messaging", entry["service"])
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
		h := adapter.NewHealthHandler(checker, logging.New("messaging", &output))
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", logID))
		_, err := h.Check(ctx, &healthpb.HealthCheckRequest{})
		require.Equal(t, codes.Internal, status.Code(err))
		require.Equal(t, "health check failed", status.Convert(err).Message())
		assertLog(t, &output, healthpb.Health_Check_FullMethodName)
	}
}
