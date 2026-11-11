package main

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
)

const (
	stubJobID      = "33333333-3333-4333-8333-333333333333"
	stubDraftID    = "55555555-5555-4555-8555-555555555555"
	stubEmployerID = "11111111-1111-4111-8111-111111111111"
)

// stubJobs is a fixed repository for wiring tests; the real one is covered
// by the Testcontainers integration test. stubDraftID is a DRAFT job and
// every other ID is a PUBLISHED job, all owned by stubEmployerID.
type stubJobs struct{}

func stubJob() domain.Job {
	return domain.Job{ID: stubJobID, EmployerID: stubEmployerID, Status: domain.StatusPublished,
		JobFields: domain.JobFields{Title: "Stub", TagIDs: []string{}, Deadline: time.Now().Add(24 * time.Hour)}}
}

func (stubJobs) Create(_ context.Context, j domain.Job) (domain.Job, error) {
	j.ID = stubJobID
	return j, nil
}

func (stubJobs) GetByID(_ context.Context, id string) (domain.Job, error) {
	j := stubJob()
	if id == stubDraftID {
		j.ID, j.Status = stubDraftID, domain.StatusDraft
	}
	return j, nil
}

func (stubJobs) UpdateFields(_ context.Context, j domain.Job, _ domain.Status) (domain.Job, error) {
	return j, nil
}

func (stubJobs) UpdateStatus(context.Context, string, domain.Status, domain.Status, time.Time) (domain.Job, error) {
	return stubJob(), nil
}

func (stubJobs) ListByEmployer(context.Context, string, domain.Status, domain.Page) ([]domain.Job, int, error) {
	return []domain.Job{stubJob()}, 1, nil
}

func (stubJobs) ListClosingBetween(context.Context, domain.ClosingWindow, domain.Page) ([]domain.Job, int, error) {
	return []domain.Job{stubJob()}, 1, nil
}

// stubVerifier answers IsVerified with a value each subtest can change.
type stubVerifier struct{ verified atomic.Bool }

func (v *stubVerifier) IsVerified(context.Context, string) (bool, error) {
	return v.verified.Load(), nil
}

// recordingRegistrar keeps what registerJobPosting registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterJobPosting_RegistersPublicAndInternalServices(t *testing.T) {
	var reg recordingRegistrar
	registerJobPosting(stubJobs{}, &stubVerifier{})(&reg)
	if len(reg.descs) != 2 ||
		reg.descs[0].ServiceName != "jobtrack.job_posting.v1.JobPostingService" ||
		reg.descs[1].ServiceName != "jobtrack.job_posting.v1.JobPostingInternalService" {
		t.Fatalf("registered %v, want JobPostingService and JobPostingInternalService", reg.descs)
	}
	if _, ok := reg.impls[0].(jobpostingv1.JobPostingServiceServer); !ok {
		t.Fatalf("registered %T, not a JobPostingServiceServer", reg.impls[0])
	}
	if _, ok := reg.impls[1].(jobpostingv1.JobPostingInternalServiceServer); !ok {
		t.Fatalf("registered %T, not a JobPostingInternalServiceServer", reg.impls[1])
	}
}

func TestInternalServerOptions_RejectsInvalidTokens(t *testing.T) {
	for _, token := range []string{"", "private-secret value", "private-secreté"} {
		opts, err := internalServerOptions(token)
		if err == nil || opts != nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("token %q: opts=%v err=%v", token, opts, err)
		}
	}
	if opts, err := internalServerOptions("valid-token"); err != nil || len(opts) != 1 {
		t.Fatalf("valid token: opts=%v err=%v", opts, err)
	}
}

// End to end over a real gRPC server with the real JT-44 interceptor, as
// run wires it: only deadline-reminder with the right token reaches the
// internal RPC, and the public RPCs are unchanged.
func TestServe_InternalInterceptorGuardsOnlyTheInternalService(t *testing.T) {
	const token = "test-internal-token"
	// The same options run passes to the server.
	serverOpts, err := internalServerOptions(token)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &stubVerifier{}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerJobPosting(stubJobs{}, verifier),
			grpcListener, httpListener, serverOpts...)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})

	dial := func(t *testing.T, interceptor grpc.UnaryClientInterceptor) *grpc.ClientConn {
		t.Helper()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if interceptor != nil {
			opts = append(opts, grpc.WithChainUnaryInterceptor(interceptor))
		}
		conn, err := grpc.NewClient(grpcListener.Addr().String(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	internalClient := func(t *testing.T, token, caller string) *grpc.ClientConn {
		t.Helper()
		interceptor, err := grpcclient.InternalUnaryClientInterceptor(token, caller)
		if err != nil {
			t.Fatal(err)
		}
		return dial(t, interceptor)
	}
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	internalReq := &jobpostingv1.ListJobsClosingBetweenRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(48 * time.Hour))}
	rpcCtx := func(t *testing.T) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return c
	}

	for _, tt := range []struct {
		name string
		conn func(t *testing.T) *grpc.ClientConn
		want codes.Code
	}{
		{"deadline-reminder with the right token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, token, "deadline-reminder") }, codes.OK},
		{"wrong token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, "wrong-token", "deadline-reminder") }, codes.Unauthenticated},
		{"another internal caller", func(t *testing.T) *grpc.ClientConn {
			return internalClient(t, token, "weekly-summary")
		}, codes.PermissionDenied},
		{"public client without a token", func(t *testing.T) *grpc.ClientConn { return dial(t, grpcclient.UnaryClientInterceptor()) }, codes.Unauthenticated},
	} {
		t.Run("ListJobsClosingBetween: "+tt.name, func(t *testing.T) {
			resp, err := jobpostingv1.NewJobPostingInternalServiceClient(tt.conn(t)).ListJobsClosingBetween(rpcCtx(t), internalReq)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && len(resp.GetJobs()) != 1 {
				t.Fatalf("jobs %v, want the one stub job", resp.GetJobs())
			}
		})
	}

	// The public RPCs need no internal token. These headers stand in for the
	// identity the gateway forwards.
	asRole := func(t *testing.T, role string) context.Context {
		return metadata.AppendToOutgoingContext(rpcCtx(t), "x-user-id", stubEmployerID, "x-user-role", role)
	}
	public := func(t *testing.T) jobpostingv1.JobPostingServiceClient {
		return jobpostingv1.NewJobPostingServiceClient(dial(t, nil))
	}
	t.Run("GetJob as applicant", func(t *testing.T) {
		resp, err := public(t).GetJob(asRole(t, "applicant"), &jobpostingv1.GetJobRequest{JobId: stubJobID})
		if status.Code(err) != codes.OK || resp.GetJob().GetStatus() != jobpostingv1.JobStatus_JOB_STATUS_PUBLISHED {
			t.Fatalf("resp=%v err=%v, want the published stub job", resp, err)
		}
	})
	t.Run("ListMyJobs as applicant", func(t *testing.T) {
		_, err := public(t).ListMyJobs(asRole(t, "applicant"), &jobpostingv1.ListMyJobsRequest{})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code=%v (%v), want PermissionDenied", status.Code(err), err)
		}
	})
	// The verifier passed to registerJobPosting decides PublishJob (JT-57).
	t.Run("PublishJob as an unverified employer", func(t *testing.T) {
		verifier.verified.Store(false)
		_, err := public(t).PublishJob(asRole(t, "employer"), &jobpostingv1.PublishJobRequest{JobId: stubDraftID})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("code=%v (%v), want FailedPrecondition", status.Code(err), err)
		}
	})
	t.Run("PublishJob as a verified employer", func(t *testing.T) {
		verifier.verified.Store(true)
		_, err := public(t).PublishJob(asRole(t, "employer"), &jobpostingv1.PublishJobRequest{JobId: stubDraftID})
		if status.Code(err) != codes.OK {
			t.Fatalf("code=%v (%v), want OK", status.Code(err), err)
		}
	})
}

// verifiedEmployer is a fake EmployerService that reports every profile as
// VERIFIED and records the identity it received.
type verifiedEmployer struct {
	employerv1.UnimplementedEmployerServiceServer
	gotUserID chan string
}

func (e *verifiedEmployer) GetCompanyProfile(ctx context.Context, req *employerv1.GetCompanyProfileRequest) (*employerv1.GetCompanyProfileResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	e.gotUserID <- strings.Join(md.Get("x-user-id"), ",")
	return &employerv1.GetCompanyProfileResponse{Profile: &employerv1.CompanyProfile{
		EmployerId:         req.GetEmployerId(),
		VerificationStatus: employerv1.VerificationStatus_VERIFICATION_STATUS_VERIFIED,
	}}, nil
}

// The verifier run builds reaches EmployerService and forwards the caller.
func TestNewEmployerVerifier_CallsEmployerWithCallerIdentity(t *testing.T) {
	l := listenForTest(t)
	fake := &verifiedEmployer{gotUserID: make(chan string, 1)}
	s := grpc.NewServer()
	employerv1.RegisterEmployerServiceServer(s, fake)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)

	verifier, closeEmployer, err := newEmployerVerifier(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeEmployer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-user-id", stubEmployerID, "x-user-role", "employer"))
	verified, err := verifier.IsVerified(ctx, stubEmployerID)
	if err != nil || !verified {
		t.Fatalf("verified=%v err=%v, want true and no error", verified, err)
	}
	if got := <-fake.gotUserID; got != stubEmployerID {
		t.Fatalf("employer received x-user-id %q, want %q", got, stubEmployerID)
	}
}
