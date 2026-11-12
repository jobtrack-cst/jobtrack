package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

const (
	stubResumeID    = "33333333-3333-4333-8333-333333333333"
	stubApplicantID = "11111111-1111-4111-8111-111111111111"
)

var stubContent = []byte("%PDF-1.7 stub")

// stubResumes is a fixed repository for wiring tests; the real one is covered
// by the Testcontainers integration test. Every ID is a PENDING resume owned
// by stubApplicantID.
type stubResumes struct{}

func stubResume() domain.Resume {
	return domain.Resume{ID: stubResumeID, ApplicantID: stubApplicantID, FileName: "cv.pdf",
		ContentType: "application/pdf", SizeBytes: int64(len(stubContent)),
		ValidationStatus: domain.ValidationStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func (stubResumes) Create(_ context.Context, f domain.ResumeFile, event *eventsv1.EventEnvelope) (domain.Resume, error) {
	return f.Resume, nil
}

func (stubResumes) GetByID(context.Context, string) (domain.Resume, error) {
	return stubResume(), nil
}

func (stubResumes) GetFile(context.Context, string) (domain.ResumeFile, error) {
	return domain.ResumeFile{Resume: stubResume(), Content: stubContent}, nil
}

func (stubResumes) ListByApplicant(context.Context, string, domain.Page) ([]domain.Resume, int, error) {
	return []domain.Resume{stubResume()}, 1, nil
}

// recordingRegistrar keeps what registerResume registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterResume_RegistersPublicAndInternalServices(t *testing.T) {
	var reg recordingRegistrar
	registerResume(stubResumes{})(&reg)
	if len(reg.descs) != 2 ||
		reg.descs[0].ServiceName != "jobtrack.resume.v1.ResumeService" ||
		reg.descs[1].ServiceName != "jobtrack.resume.v1.ResumeInternalService" {
		t.Fatalf("registered %v, want ResumeService and ResumeInternalService", reg.descs)
	}
	if _, ok := reg.impls[0].(resumev1.ResumeServiceServer); !ok {
		t.Fatalf("registered %T, not a ResumeServiceServer", reg.impls[0])
	}
	if _, ok := reg.impls[1].(resumev1.ResumeInternalServiceServer); !ok {
		t.Fatalf("registered %T, not a ResumeInternalServiceServer", reg.impls[1])
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

// End to end over a real gRPC server with the real JT-44 interceptor, as run
// wires it: only resume-validator with the right token reaches GetResumeFile,
// and the public RPCs are unchanged.
func TestServe_InternalInterceptorGuardsOnlyTheInternalService(t *testing.T) {
	const token = "test-internal-token"
	// The same options run passes to the server.
	serverOpts, err := internalServerOptions(token)
	if err != nil {
		t.Fatal(err)
	}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerResume(stubResumes{}),
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
		{"resume-validator with the right token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, token, "resume-validator") }, codes.OK},
		{"wrong token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, "wrong-token", "resume-validator") }, codes.Unauthenticated},
		{"another internal caller", func(t *testing.T) *grpc.ClientConn {
			return internalClient(t, token, "notification-dispatcher")
		}, codes.PermissionDenied},
		{"public client without a token", func(t *testing.T) *grpc.ClientConn { return dial(t, grpcclient.UnaryClientInterceptor()) }, codes.Unauthenticated},
	} {
		t.Run("GetResumeFile: "+tt.name, func(t *testing.T) {
			resp, err := resumev1.NewResumeInternalServiceClient(tt.conn(t)).GetResumeFile(rpcCtx(t), &resumev1.GetResumeFileRequest{ResumeId: stubResumeID})
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && string(resp.GetContent()) != string(stubContent) {
				t.Fatalf("content %q, want the stub file", resp.GetContent())
			}
		})
	}

	// The public RPCs need no internal token. These headers stand in for the
	// identity the gateway forwards.
	asRole := func(t *testing.T, role string) context.Context {
		return metadata.AppendToOutgoingContext(rpcCtx(t), "x-user-id", stubApplicantID, "x-user-role", role)
	}
	public := func(t *testing.T) resumev1.ResumeServiceClient {
		return resumev1.NewResumeServiceClient(dial(t, nil))
	}
	t.Run("GetResume as the owning applicant", func(t *testing.T) {
		resp, err := public(t).GetResume(asRole(t, "applicant"), &resumev1.GetResumeRequest{ResumeId: stubResumeID})
		if status.Code(err) != codes.OK || resp.GetResume().GetValidationStatus() != resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_PENDING {
			t.Fatalf("resp=%v err=%v, want the pending stub resume", resp, err)
		}
	})
	t.Run("ListMyResumes as employer", func(t *testing.T) {
		_, err := public(t).ListMyResumes(asRole(t, "employer"), &resumev1.ListMyResumesRequest{})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code=%v (%v), want PermissionDenied", status.Code(err), err)
		}
	})
	// A 2,000,000-byte upload fits in one gRPC message under the default 4 MB limit.
	t.Run("UploadResume at the 2,000,000 byte limit", func(t *testing.T) {
		content := append([]byte("%PDF-"), make([]byte, 2_000_000-5)...)
		resp, err := public(t).UploadResume(asRole(t, "applicant"), &resumev1.UploadResumeRequest{
			FileName: "cv.pdf", ContentType: "application/pdf", Content: content,
		})
		if status.Code(err) != codes.OK || resp.GetResume().GetSizeBytes() != 2_000_000 {
			t.Fatalf("resp=%v err=%v, want a stored 2,000,000 byte resume", resp.GetResume(), err)
		}
	})
}
