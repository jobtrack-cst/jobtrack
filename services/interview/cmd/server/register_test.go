package main

import (
	"context"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	interviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/interview/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/interview/internal/domain"
)

const (
	stubApplicationID = "33333333-3333-4333-8333-333333333333"
	stubEmployerID    = "22222222-2222-4222-8222-222222222222"
	stubApplicantID   = "11111111-1111-4111-8111-111111111111"
)

// stubParticipants knows one application; the real repository is covered by
// the Testcontainers integration test.
type stubParticipants struct{}

func (stubParticipants) GetByApplicationID(_ context.Context, id string) (domain.ApplicationParticipants, error) {
	if id != stubApplicationID {
		return domain.ApplicationParticipants{}, domain.ErrApplicationNotFound
	}
	return domain.ApplicationParticipants{ApplicationID: id, ApplicantID: stubApplicantID, EmployerID: stubEmployerID}, nil
}

// stubInterviews keeps interviews in memory.
type stubInterviews struct {
	mu   sync.Mutex
	byID map[string]domain.Interview
}

func (s *stubInterviews) Create(_ context.Context, iv domain.Interview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]domain.Interview{}
	}
	for _, other := range s.byID {
		if other.ApplicationID == iv.ApplicationID && other.ScheduledAt.Equal(iv.ScheduledAt) {
			return domain.ErrInterviewExists
		}
	}
	s.byID[iv.InterviewID] = iv
	return nil
}

func (s *stubInterviews) Get(_ context.Context, id string) (domain.Interview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	iv, ok := s.byID[id]
	if !ok {
		return domain.Interview{}, domain.ErrInterviewNotFound
	}
	return iv, nil
}

func (s *stubInterviews) ListByApplication(_ context.Context, applicationID string, _ domain.Page) ([]domain.Interview, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Interview{}
	for _, iv := range s.byID {
		if iv.ApplicationID == applicationID {
			out = append(out, iv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScheduledAt.Before(out[j].ScheduledAt) })
	return out, len(out), nil
}

// recordingRegistrar keeps what registerInterview registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterInterview_RegistersOnlyThePublicService(t *testing.T) {
	var reg recordingRegistrar
	registerInterview(stubParticipants{}, &stubInterviews{})(&reg)
	if len(reg.descs) != 1 || reg.descs[0].ServiceName != "jobtrack.interview.v1.InterviewService" {
		t.Fatalf("registered %v, want only InterviewService", reg.descs)
	}
	if _, ok := reg.impls[0].(interviewv1.InterviewServiceServer); !ok {
		t.Fatalf("registered %T, not an InterviewServiceServer", reg.impls[0])
	}
}

func TestSystemClock_IsUTC(t *testing.T) {
	if loc := (systemClock{}).Now().Location(); loc != time.UTC {
		t.Fatalf("location %v, want UTC", loc)
	}
}

// End to end over a real gRPC server with the real clock, as run wires it.
func TestServe_InterviewOverGRPC(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerInterview(stubParticipants{}, &stubInterviews{}),
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
	client := interviewv1.NewInterviewServiceClient(conn)
	as := func(t *testing.T, id, role string) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return metadata.AppendToOutgoingContext(c, "x-user-id", id, "x-user-role", role)
	}
	at := timestamppb.New(time.Now().Add(72 * time.Hour))
	req := &interviewv1.ScheduleInterviewRequest{ApplicationId: stubApplicationID, ScheduledAt: at, Details: "Room 3"}

	past := &interviewv1.ScheduleInterviewRequest{ApplicationId: stubApplicationID, ScheduledAt: timestamppb.New(time.Now().Add(-time.Hour))}
	if _, err := client.ScheduleInterview(as(t, stubEmployerID, "employer"), past); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("past time: %v, want InvalidArgument", err)
	}
	created, err := client.ScheduleInterview(as(t, stubEmployerID, "employer"), req)
	if err != nil || created.GetInterview().GetApplicantId() != stubApplicantID {
		t.Fatalf("schedule: resp=%v err=%v", created, err)
	}
	if _, err := client.ScheduleInterview(as(t, stubEmployerID, "employer"), req); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("same time: %v, want AlreadyExists", err)
	}
	got, err := client.GetInterview(as(t, stubApplicantID, "applicant"), &interviewv1.GetInterviewRequest{InterviewId: created.GetInterview().GetInterviewId()})
	if err != nil || got.GetInterview().GetDetails() != "Room 3" {
		t.Fatalf("applicant get: resp=%v err=%v", got, err)
	}
	list, err := client.ListInterviews(as(t, stubEmployerID, "employer"), &interviewv1.ListInterviewsRequest{ApplicationId: stubApplicationID})
	if err != nil || len(list.GetInterviews()) != 1 {
		t.Fatalf("list: resp=%v err=%v", list, err)
	}
	if _, err := client.ScheduleInterview(as(t, stubApplicantID, "applicant"), req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("applicant schedule: %v, want PermissionDenied", err)
	}
}
