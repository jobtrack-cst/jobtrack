package main

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	reviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/review/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/review/internal/domain"
)

const (
	stubApplicationID = "33333333-3333-4333-8333-333333333333"
	stubEmployerID    = "22222222-2222-4222-8222-222222222222"
	stubApplicantID   = "11111111-1111-4111-8111-111111111111"
)

// stubParticipants knows one application owned by stubEmployerID; the real
// repository is covered by the Testcontainers integration test.
type stubParticipants struct{}

func (stubParticipants) GetByApplicationID(_ context.Context, id string) (domain.ApplicationParticipants, error) {
	if id != stubApplicationID {
		return domain.ApplicationParticipants{}, domain.ErrApplicationNotFound
	}
	return domain.ApplicationParticipants{ApplicationID: id, ApplicantID: stubApplicantID, EmployerID: stubEmployerID}, nil
}

// stubReviews keeps reviews in memory, one per application.
type stubReviews struct {
	mu      sync.Mutex
	reviews map[string]domain.Review
}

func (s *stubReviews) Create(_ context.Context, r domain.Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reviews == nil {
		s.reviews = map[string]domain.Review{}
	}
	if _, ok := s.reviews[r.ApplicationID]; ok {
		return domain.ErrReviewExists
	}
	s.reviews[r.ApplicationID] = r
	return nil
}

func (s *stubReviews) GetByApplicationID(_ context.Context, id string) (domain.Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.reviews[id]
	if !ok {
		return domain.Review{}, domain.ErrReviewNotFound
	}
	return r, nil
}

// recordingRegistrar keeps what registerReview registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterReview_RegistersOnlyThePublicService(t *testing.T) {
	var reg recordingRegistrar
	registerReview(stubParticipants{}, &stubReviews{})(&reg)
	if len(reg.descs) != 1 || reg.descs[0].ServiceName != "jobtrack.review.v1.ReviewService" {
		t.Fatalf("registered %v, want only ReviewService", reg.descs)
	}
	if _, ok := reg.impls[0].(reviewv1.ReviewServiceServer); !ok {
		t.Fatalf("registered %T, not a ReviewServiceServer", reg.impls[0])
	}
}

func TestSystemClock_IsUTC(t *testing.T) {
	if loc := (systemClock{}).Now().Location(); loc != time.UTC {
		t.Fatalf("location %v, want UTC", loc)
	}
}

// End to end over a real gRPC server, as run wires it.
func TestServe_ReviewOverGRPC(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerReview(stubParticipants{}, &stubReviews{}),
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
	client := reviewv1.NewReviewServiceClient(conn)
	as := func(t *testing.T, id, role string) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return metadata.AppendToOutgoingContext(c, "x-user-id", id, "x-user-role", role)
	}
	req := &reviewv1.CompleteReviewRequest{ApplicationId: stubApplicationID, Score: 5, Decision: reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED}

	if _, err := client.GetReview(as(t, stubEmployerID, "employer"), &reviewv1.GetReviewRequest{ApplicationId: stubApplicationID}); status.Code(err) != codes.NotFound {
		t.Fatalf("get before review: %v, want NotFound", err)
	}
	if _, err := client.CompleteReview(as(t, "44444444-4444-4444-8444-444444444444", "employer"), req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("other employer: %v, want PermissionDenied", err)
	}
	created, err := client.CompleteReview(as(t, stubEmployerID, "employer"), req)
	if err != nil || created.GetReview().GetScore() != 5 {
		t.Fatalf("complete: resp=%v err=%v", created, err)
	}
	if _, err := client.CompleteReview(as(t, stubEmployerID, "employer"), req); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("second review: %v, want AlreadyExists", err)
	}
	got, err := client.GetReview(as(t, stubEmployerID, "employer"), &reviewv1.GetReviewRequest{ApplicationId: stubApplicationID})
	if err != nil || got.GetReview().GetDecision() != reviewv1.ReviewDecision_REVIEW_DECISION_SHORTLISTED {
		t.Fatalf("get: resp=%v err=%v", got, err)
	}
	if _, err := client.GetReview(as(t, stubApplicantID, "applicant"), &reviewv1.GetReviewRequest{ApplicationId: stubApplicationID}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("applicant get: %v, want PermissionDenied", err)
	}
}
