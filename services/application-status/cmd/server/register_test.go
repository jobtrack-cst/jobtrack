package main

import (
	"context"
	"io"
	"testing"
	"time"

	asv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application-status/internal/domain"
)

const (
	stubApplicationID = "33333333-3333-4333-8333-333333333333"
	stubApplicantID   = "11111111-1111-4111-8111-111111111111"
	stubEmployerID    = "22222222-2222-4222-8222-222222222222"
)

// stubStatuses is a fixed repository for wiring tests; the real one is
// covered by the Testcontainers integration test. Every ID is a SHORTLISTED
// application between stubApplicantID and stubEmployerID.
type stubStatuses struct{}

func (stubStatuses) Create(context.Context, domain.ApplicationStatus, domain.HistoryEntry) error {
	return nil
}

func (stubStatuses) Get(context.Context, string) (domain.ApplicationStatus, error) {
	return domain.ApplicationStatus{ApplicationID: stubApplicationID, ApplicantID: stubApplicantID,
		EmployerID: stubEmployerID, Status: domain.StatusShortlisted, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (stubStatuses) Transition(context.Context, domain.ApplicationStatus, domain.Status, domain.HistoryEntry) error {
	return nil
}

func (stubStatuses) ListHistory(context.Context, string, domain.Page) ([]domain.HistoryEntry, int, error) {
	return []domain.HistoryEntry{{ApplicationID: stubApplicationID, NewStatus: domain.StatusSubmitted, OccurredAt: time.Now()}}, 1, nil
}

// recordingRegistrar keeps what registerApplicationStatus registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterApplicationStatus_RegistersOnlyThePublicService(t *testing.T) {
	var reg recordingRegistrar
	registerApplicationStatus(stubStatuses{})(&reg)
	if len(reg.descs) != 1 || reg.descs[0].ServiceName != "jobtrack.application_status.v1.ApplicationStatusService" {
		t.Fatalf("registered %v, want only ApplicationStatusService", reg.descs)
	}
	if _, ok := reg.impls[0].(asv1.ApplicationStatusServiceServer); !ok {
		t.Fatalf("registered %T, not an ApplicationStatusServiceServer", reg.impls[0])
	}
}

// End to end over a real gRPC server, as run wires it.
func TestServe_ApplicationStatusOverGRPC(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerApplicationStatus(stubStatuses{}),
			grpcListener, httpListener)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := asv1.NewApplicationStatusServiceClient(conn)
	as := func(t *testing.T, id, role string) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return metadata.AppendToOutgoingContext(c, "x-user-id", id, "x-user-role", role)
	}

	t.Run("applicant reads the status", func(t *testing.T) {
		resp, err := client.GetApplicationStatus(as(t, stubApplicantID, "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: stubApplicationID})
		if err != nil || resp.GetStatusRecord().GetStatus() != asv1.ApplicationStatus_APPLICATION_STATUS_SHORTLISTED {
			t.Fatalf("resp=%v err=%v, want SHORTLISTED", resp, err)
		}
	})
	t.Run("employer reads the history", func(t *testing.T) {
		resp, err := client.ListApplicationStatusHistory(as(t, stubEmployerID, "employer"), &asv1.ListApplicationStatusHistoryRequest{ApplicationId: stubApplicationID})
		if err != nil || len(resp.GetEntries()) != 1 || resp.GetPagination().GetTotalCount() != 1 {
			t.Fatalf("resp=%v err=%v, want one entry", resp, err)
		}
	})
	t.Run("admin is denied", func(t *testing.T) {
		_, err := client.GetApplicationStatus(as(t, stubApplicantID, "admin"), &asv1.GetApplicationStatusRequest{ApplicationId: stubApplicationID})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code=%v (%v), want PermissionDenied", status.Code(err), err)
		}
	})
	t.Run("another applicant is denied", func(t *testing.T) {
		_, err := client.GetApplicationStatus(as(t, "44444444-4444-4444-8444-444444444444", "applicant"), &asv1.GetApplicationStatusRequest{ApplicationId: stubApplicationID})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code=%v (%v), want PermissionDenied", status.Code(err), err)
		}
	})
}
