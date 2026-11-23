package main

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	apv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

const (
	stubApplicantID   = "11111111-1111-4111-8111-111111111111"
	stubApplicationID = "44444444-4444-4444-8444-444444444444"
	stubEmployerID    = "55555555-5555-4555-8555-555555555555"
)

// stubParticipants knows one application of stubApplicantID, owned by
// stubEmployerID.
type stubParticipants struct{}

func (stubParticipants) Record(context.Context, string, domain.ApplicationParticipant) error {
	return nil
}

func (stubParticipants) GetByApplicationID(_ context.Context, id string) (domain.ApplicationParticipant, error) {
	if id != stubApplicationID {
		return domain.ApplicationParticipant{}, domain.ErrApplicationNotFound
	}
	return domain.ApplicationParticipant{ApplicationID: id, ApplicantID: stubApplicantID, EmployerID: stubEmployerID}, nil
}

// stubProfiles is an in-memory repository for wiring tests; the real one is
// covered by the Testcontainers integration test.
type stubProfiles struct {
	mu       sync.Mutex
	profiles map[string]domain.Profile
	photos   map[string]domain.Photo
}

func (s *stubProfiles) Create(_ context.Context, p domain.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profiles == nil {
		s.profiles = map[string]domain.Profile{}
	}
	if _, ok := s.profiles[p.ApplicantID]; ok {
		return domain.ErrProfileExists
	}
	s.profiles[p.ApplicantID] = p
	return nil
}

func (s *stubProfiles) Get(_ context.Context, id string) (domain.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return p, nil
}

func (s *stubProfiles) Replace(_ context.Context, id string, c domain.ProfileContent, at time.Time) (domain.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	p.ProfileContent, p.UpdatedAt = c, at
	s.profiles[id] = p
	return p, nil
}

// PutPhoto and GetPhoto keep the stub a full ports.ProfileRepository (JT-103);
// the photo rules are tested in the domain, app, gRPC and postgres packages.
func (s *stubProfiles) PutPhoto(_ context.Context, id string, photo domain.Photo, at time.Time) (domain.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if s.photos == nil {
		s.photos = map[string]domain.Photo{}
	}
	s.photos[id] = photo
	p.HasPhoto, p.UpdatedAt = true, at
	s.profiles[id] = p
	return p, nil
}

func (s *stubProfiles) GetPhoto(_ context.Context, id string) (domain.Photo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	photo, ok := s.photos[id]
	if !ok {
		return domain.Photo{}, domain.ErrPhotoNotFound
	}
	return photo, nil
}

// recordingRegistrar keeps what registerApplicantProfile registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterApplicantProfile_RegistersOnlyThePublicService(t *testing.T) {
	var reg recordingRegistrar
	registerApplicantProfile(&stubProfiles{}, stubParticipants{})(&reg)
	if len(reg.descs) != 1 || reg.descs[0].ServiceName != "jobtrack.applicant_profile.v1.ApplicantProfileService" {
		t.Fatalf("registered %v, want only ApplicantProfileService", reg.descs)
	}
	if _, ok := reg.impls[0].(apv1.ApplicantProfileServiceServer); !ok {
		t.Fatalf("registered %T, not an ApplicantProfileServiceServer", reg.impls[0])
	}
}

func TestSystemClock_IsUTC(t *testing.T) {
	if loc := (systemClock{}).Now().Location(); loc != time.UTC {
		t.Fatalf("location %v, want UTC", loc)
	}
}

// End to end over a real gRPC server, as run wires it.
func TestServe_ApplicantProfileOverGRPC(t *testing.T) {
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerApplicantProfile(&stubProfiles{}, stubParticipants{}),
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
	client := apv1.NewApplicantProfileServiceClient(conn)
	asUser := func(t *testing.T, id, role string) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return metadata.AppendToOutgoingContext(c, "x-user-id", id, "x-user-role", role)
	}
	as := func(t *testing.T, role string) context.Context { return asUser(t, stubApplicantID, role) }

	if _, err := client.GetMyProfile(as(t, "applicant"), &apv1.GetMyProfileRequest{}); status.Code(err) != codes.NotFound {
		t.Fatalf("get before create: %v, want NotFound", err)
	}
	created, err := client.CreateMyProfile(as(t, "applicant"), &apv1.CreateMyProfileRequest{
		Summary: "Hello", Skills: []string{"Go"},
		Education: []*apv1.Education{{Institution: "CST", Qualification: "BE", StartYear: 2023}},
	})
	if err != nil || created.GetProfile().GetApplicantId() != stubApplicantID {
		t.Fatalf("create: resp=%v err=%v", created, err)
	}
	if _, err := client.CreateMyProfile(as(t, "applicant"), &apv1.CreateMyProfileRequest{}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("second create: %v, want AlreadyExists", err)
	}
	updated, err := client.UpdateMyProfile(as(t, "applicant"), &apv1.UpdateMyProfileRequest{Summary: "Updated"})
	if err != nil || updated.GetProfile().GetSummary() != "Updated" || len(updated.GetProfile().GetSkills()) != 0 {
		t.Fatalf("update: resp=%v err=%v, want replaced profile", updated, err)
	}
	got, err := client.GetMyProfile(as(t, "applicant"), &apv1.GetMyProfileRequest{})
	if err != nil || got.GetProfile().GetSummary() != "Updated" {
		t.Fatalf("get: resp=%v err=%v", got, err)
	}
	if _, err := client.UpdateMyProfile(as(t, "employer"), &apv1.UpdateMyProfileRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("employer update: %v, want PermissionDenied", err)
	}

	// JT-114: the owning employer reads the applicant's current profile; the
	// public RPCs need no internal token.
	asEmployer := func(t *testing.T) context.Context { return asUser(t, stubEmployerID, "employer") }
	forApp, err := client.GetApplicantProfileForApplication(asEmployer(t),
		&apv1.GetApplicantProfileForApplicationRequest{ApplicationId: stubApplicationID})
	if err != nil || forApp.GetProfile().GetSummary() != "Updated" || forApp.GetProfile().GetApplicantId() != stubApplicantID {
		t.Fatalf("employer profile read: resp=%v err=%v", forApp, err)
	}
	if _, err := client.GetApplicantPhotoForApplication(asEmployer(t),
		&apv1.GetApplicantPhotoForApplicationRequest{ApplicationId: stubApplicationID}); status.Code(err) != codes.NotFound {
		t.Fatalf("employer photo read without a photo: %v, want NotFound", err)
	}
	if _, err := client.GetApplicantProfileForApplication(as(t, "applicant"),
		&apv1.GetApplicantProfileForApplicationRequest{ApplicationId: stubApplicationID}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("applicant read for an application: %v, want PermissionDenied", err)
	}
}
