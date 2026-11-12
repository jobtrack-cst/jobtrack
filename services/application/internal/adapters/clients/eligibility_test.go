package clients_test

import (
	"context"
	"net"
	"testing"
	"time"

	jobv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/adapters/clients"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const id = "11111111-1111-4111-8111-111111111111"
const owner = "22222222-2222-4222-8222-222222222222"

type jobServer struct {
	jobv1.UnimplementedJobPostingServiceServer
	get func(context.Context, *jobv1.GetJobRequest) (*jobv1.GetJobResponse, error)
}

func (s jobServer) GetJob(c context.Context, r *jobv1.GetJobRequest) (*jobv1.GetJobResponse, error) {
	return s.get(c, r)
}

type resumeServer struct {
	resumev1.UnimplementedResumeServiceServer
	get func(context.Context, *resumev1.GetResumeRequest) (*resumev1.GetResumeResponse, error)
}

func (s resumeServer) GetResume(c context.Context, r *resumev1.GetResumeRequest) (*resumev1.GetResumeResponse, error) {
	return s.get(c, r)
}
func connection(t *testing.T, j jobServer, r resumeServer) *grpc.ClientConn {
	t.Helper()
	l := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	jobv1.RegisterJobPostingServiceServer(s, j)
	resumev1.RegisterResumeServiceServer(s, r)
	go s.Serve(l)
	t.Cleanup(s.Stop)
	c, err := grpc.NewClient("passthrough:///eligibility", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return l.Dial() }), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}
func caller() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", owner, "x-user-role", "applicant"))
}
func identity(t *testing.T, c context.Context) {
	t.Helper()
	m, _ := metadata.FromIncomingContext(c)
	require.Equal(t, []string{owner}, m.Get("x-user-id"))
	require.Equal(t, []string{"applicant"}, m.Get("x-user-role"))
	require.Empty(t, m.Get("x-internal-token"))
}
func TestJobPosting_GetJob(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mutate    func(*jobv1.Job)
		code      codes.Code
		published bool
	}{
		{"published", func(*jobv1.Job) {}, codes.OK, true},
		{"closed", func(j *jobv1.Job) { j.Status = jobv1.JobStatus_JOB_STATUS_CLOSED }, codes.OK, false},
		{"draft", func(j *jobv1.Job) { j.Status = jobv1.JobStatus_JOB_STATUS_DRAFT }, codes.OK, false},
		{"removed", func(j *jobv1.Job) { j.Status = jobv1.JobStatus_JOB_STATUS_REMOVED }, codes.OK, false},
		{"unknown", func(j *jobv1.Job) { j.Status = 99 }, codes.Internal, false},
		{"unspecified", func(j *jobv1.Job) { j.Status = 0 }, codes.Internal, false},
		{"wrong id", func(j *jobv1.Job) { j.JobId = owner }, codes.Internal, false},
		{"invalid employer", func(j *jobv1.Job) { j.EmployerId = "" }, codes.Internal, false},
		{"missing deadline", func(j *jobv1.Job) { j.Deadline = nil }, codes.Internal, false},
		{"invalid deadline", func(j *jobv1.Job) { j.Deadline = &timestamppb.Timestamp{Seconds: 1 << 62} }, codes.Internal, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			j := &jobv1.Job{JobId: id, EmployerId: owner, Status: jobv1.JobStatus_JOB_STATUS_PUBLISHED, Deadline: timestamppb.New(time.Now().Add(time.Hour))}
			tt.mutate(j)
			c := connection(t, jobServer{get: func(ctx context.Context, r *jobv1.GetJobRequest) (*jobv1.GetJobResponse, error) {
				identity(t, ctx)
				require.Equal(t, id, r.JobId)
				return &jobv1.GetJobResponse{Job: j}, nil
			}}, resumeServer{})
			got, err := clients.NewJobPosting(jobv1.NewJobPostingServiceClient(c)).GetJob(caller(), id)
			require.Equal(t, tt.code, status.Code(err))
			if err == nil {
				require.Equal(t, tt.published, got.Published)
				require.Equal(t, j.Deadline.AsTime(), got.Deadline)
			}
		})
	}
}
func TestResume_GetResume(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mutate    func(*resumev1.Resume)
		code      codes.Code
		validated bool
	}{
		{"validated", func(*resumev1.Resume) {}, codes.OK, true},
		{"pending", func(r *resumev1.Resume) {
			r.ValidationStatus = resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_PENDING
		}, codes.OK, false},
		{"rejected", func(r *resumev1.Resume) {
			r.ValidationStatus = resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED
		}, codes.OK, false},
		{"unknown", func(r *resumev1.Resume) { r.ValidationStatus = 99 }, codes.Internal, false},
		{"unspecified", func(r *resumev1.Resume) { r.ValidationStatus = 0 }, codes.Internal, false},
		{"wrong id", func(r *resumev1.Resume) { r.ResumeId = owner }, codes.Internal, false},
		{"invalid owner", func(r *resumev1.Resume) { r.ApplicantId = "" }, codes.Internal, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &resumev1.Resume{ResumeId: id, ApplicantId: owner, ValidationStatus: resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED}
			tt.mutate(r)
			c := connection(t, jobServer{}, resumeServer{get: func(ctx context.Context, q *resumev1.GetResumeRequest) (*resumev1.GetResumeResponse, error) {
				identity(t, ctx)
				require.Equal(t, id, q.ResumeId)
				return &resumev1.GetResumeResponse{Resume: r}, nil
			}})
			got, err := clients.NewResume(resumev1.NewResumeServiceClient(c)).GetResume(caller(), id)
			require.Equal(t, tt.code, status.Code(err))
			if err == nil {
				require.Equal(t, tt.validated, got.Validated)
				require.Equal(t, owner, got.ApplicantID)
			}
		})
	}
}
func TestClients_ErrorsNilResponsesAndRetries(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.PermissionDenied, codes.FailedPrecondition, codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.Internal, codes.OK} {
		t.Run(code.String(), func(t *testing.T) {
			jCalls, rCalls := 0, 0
			c := connection(t, jobServer{get: func(context.Context, *jobv1.GetJobRequest) (*jobv1.GetJobResponse, error) {
				jCalls++
				return &jobv1.GetJobResponse{}, status.Error(code, "private dependency error")
			}}, resumeServer{get: func(context.Context, *resumev1.GetResumeRequest) (*resumev1.GetResumeResponse, error) {
				rCalls++
				return &resumev1.GetResumeResponse{}, status.Error(code, "private dependency error")
			}})
			_, jerr := clients.NewJobPosting(jobv1.NewJobPostingServiceClient(c)).GetJob(caller(), id)
			_, rerr := clients.NewResume(resumev1.NewResumeServiceClient(c)).GetResume(caller(), id)
			want := code
			if code == codes.OK {
				want = codes.Internal
			}
			require.Equal(t, want, status.Code(jerr))
			require.Equal(t, want, status.Code(rerr))
			calls := 1
			if code == codes.Unavailable {
				calls = 4
			}
			require.Equal(t, calls, jCalls)
			require.Equal(t, calls, rCalls)
		})
	}
}
