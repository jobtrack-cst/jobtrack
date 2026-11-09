package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	mv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/moderation/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/moderation/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testUserID = "11111111-1111-4111-8111-111111111111"
	testJobID  = "55555555-5555-4555-8555-555555555555"
)

// stubReports satisfies the port in startup-order tests that never call it.
type stubReports struct{ ports.ReportRepository }

func TestRegisterModeration_RegistersOnlyThePublicService(t *testing.T) {
	server := grpc.NewServer()
	registerModeration(stubReports{})(server)
	info := server.GetServiceInfo()
	if _, ok := info["jobtrack.moderation.v1.ModerationService"]; !ok {
		t.Fatal("jobtrack.moderation.v1.ModerationService not registered")
	}
	if len(info) != 1 {
		t.Fatalf("registered %d services, want only ModerationService", len(info))
	}
}

func TestSystemClock_ReturnsUTC(t *testing.T) {
	if got := (systemClock{}).Now(); got.Location() != time.UTC {
		t.Fatalf("location=%v, want UTC", got.Location())
	}
}

// Proves the RPCs answer over a real gRPC connection with the identity sent
// as metadata, the way the gateway forwards it.
func TestServe_AnswersModerationRPCsWithForwardedIdentity(t *testing.T) {
	repo := mocks.NewMockReportRepository(gomock.NewController(t))
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r domain.Report) (domain.Report, error) {
			return r, nil
		})
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("moderation", &bytes.Buffer{}), app.NewHealth(), registerModeration(repo), grpcListener, httpListener)
	}()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := mv1.NewModerationServiceClient(conn)
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rpcCancel()

	asApplicant := metadata.AppendToOutgoingContext(rpcCtx, "x-user-id", testUserID, "x-user-role", "applicant")
	resp, err := client.ReportJob(asApplicant, &mv1.ReportJobRequest{JobId: testJobID, Reason: "spam"})
	if err != nil {
		t.Fatalf("ReportJob: %v", err)
	}
	if resp.GetReport().GetStatus() != mv1.ReportStatus_REPORT_STATUS_OPEN || resp.GetReport().GetReporterId() != testUserID {
		t.Errorf("unexpected report %v", resp.GetReport())
	}
	if _, err := client.ListReports(asApplicant, &mv1.ListReportsRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("ListReports as applicant code=%v, want PermissionDenied", status.Code(err))
	}
	if _, err := client.RemoveJob(asApplicant, &mv1.RemoveJobRequest{JobId: testJobID, Reason: "fake"}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("RemoveJob as applicant code=%v, want PermissionDenied", status.Code(err))
	}
	if _, err := client.ListReports(rpcCtx, &mv1.ListReportsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ListReports without identity code=%v, want Unauthenticated", status.Code(err))
	}
	cancel()
	if err := awaitServer(t, done); err != nil {
		t.Fatal(err)
	}
}
