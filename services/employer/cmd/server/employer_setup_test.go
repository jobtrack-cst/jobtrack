package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testEmployerID = "11111111-1111-4111-8111-111111111111"
	testCallerID   = "22222222-2222-4222-8222-222222222222"
)

// stubProfiles satisfies the port in startup-order tests that never call it.
type stubProfiles struct{ ports.ProfileRepository }

func TestRegisterEmployer_RegistersOnlyThePublicService(t *testing.T) {
	server := grpc.NewServer()
	registerEmployer(stubProfiles{})(server)
	info := server.GetServiceInfo()
	if _, ok := info["jobtrack.employer.v1.EmployerService"]; !ok {
		t.Fatal("jobtrack.employer.v1.EmployerService not registered")
	}
	if len(info) != 1 {
		t.Fatalf("registered %d services, want only EmployerService", len(info))
	}
}

func TestSystemClock_ReturnsUTC(t *testing.T) {
	if got := (systemClock{}).Now(); got.Location() != time.UTC {
		t.Fatalf("location=%v, want UTC", got.Location())
	}
}

// Proves GetCompanyProfile answers over a real gRPC connection with identity
// forwarded as metadata, the way job-posting will call it in Sprint 2.
func TestServe_AnswersEmployerRPCsWithForwardedIdentity(t *testing.T) {
	repo := mocks.NewMockProfileRepository(gomock.NewController(t))
	repo.EXPECT().GetByEmployerID(gomock.Any(), testEmployerID).Return(domain.CompanyProfile{
		EmployerID: testEmployerID, CompanyName: "Druk Tech", VerificationStatus: domain.VerificationVerified,
	}, nil)
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("employer", &bytes.Buffer{}), app.NewHealth(), registerEmployer(repo), grpcListener, httpListener)
	}()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := employerv1.NewEmployerServiceClient(conn)
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rpcCancel()

	forwarded := metadata.AppendToOutgoingContext(rpcCtx, "x-user-id", testCallerID, "x-user-role", "employer")
	resp, err := client.GetCompanyProfile(forwarded, &employerv1.GetCompanyProfileRequest{EmployerId: testEmployerID})
	if err != nil {
		t.Fatalf("GetCompanyProfile: %v", err)
	}
	if resp.GetProfile().GetVerificationStatus() != employerv1.VerificationStatus_VERIFICATION_STATUS_VERIFIED {
		t.Errorf("status=%v, want VERIFIED", resp.GetProfile().GetVerificationStatus())
	}
	if _, err := client.GetMyCompanyProfile(rpcCtx, &employerv1.GetMyCompanyProfileRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("GetMyCompanyProfile without identity code=%v, want Unauthenticated", status.Code(err))
	}
	cancel()
	if err := awaitServer(t, done); err != nil {
		t.Fatal(err)
	}
}
