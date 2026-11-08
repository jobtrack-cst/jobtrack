package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	cv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/company_verification/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testEmployerID = "11111111-1111-4111-8111-111111111111"

// stubRequests satisfies the port in startup-order tests that never call it.
type stubRequests struct{ ports.RequestRepository }

func TestRegisterCompanyVerification_RegistersOnlyThePublicService(t *testing.T) {
	server := grpc.NewServer()
	registerCompanyVerification(stubRequests{})(server)
	info := server.GetServiceInfo()
	if _, ok := info["jobtrack.company_verification.v1.CompanyVerificationService"]; !ok {
		t.Fatal("jobtrack.company_verification.v1.CompanyVerificationService not registered")
	}
	if len(info) != 1 {
		t.Fatalf("registered %d services, want only CompanyVerificationService", len(info))
	}
}

func TestSystemClock_ReturnsUTC(t *testing.T) {
	if got := (systemClock{}).Now(); got.Location() != time.UTC {
		t.Fatalf("location=%v, want UTC", got.Location())
	}
}

// Proves the RPCs answer over a real gRPC connection with the identity sent
// as metadata, the way the gateway forwards it.
func TestServe_AnswersVerificationRPCsWithForwardedIdentity(t *testing.T) {
	repo := mocks.NewMockRequestRepository(gomock.NewController(t))
	repo.EXPECT().StatusesByEmployer(gomock.Any(), testEmployerID).Return(nil, nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r domain.VerificationRequest) (domain.VerificationRequest, error) {
			return r, nil
		})
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("company-verification", &bytes.Buffer{}), app.NewHealth(), registerCompanyVerification(repo), grpcListener, httpListener)
	}()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := cv1.NewCompanyVerificationServiceClient(conn)
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rpcCancel()

	asEmployer := metadata.AppendToOutgoingContext(rpcCtx, "x-user-id", testEmployerID, "x-user-role", "employer")
	resp, err := client.SubmitVerificationRequest(asEmployer, &cv1.SubmitVerificationRequestRequest{RegistrationNumber: "BT-1"})
	if err != nil {
		t.Fatalf("SubmitVerificationRequest: %v", err)
	}
	if resp.GetRequest().GetStatus() != cv1.RequestStatus_REQUEST_STATUS_PENDING || resp.GetRequest().GetEmployerId() != testEmployerID {
		t.Errorf("unexpected request %v", resp.GetRequest())
	}
	if _, err := client.ListVerificationRequests(asEmployer, &cv1.ListVerificationRequestsRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("ListVerificationRequests as employer code=%v, want PermissionDenied", status.Code(err))
	}
	if _, err := client.ListVerificationRequests(rpcCtx, &cv1.ListVerificationRequestsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("ListVerificationRequests without identity code=%v, want Unauthenticated", status.Code(err))
	}
	cancel()
	if err := awaitServer(t, done); err != nil {
		t.Fatal(err)
	}
}
