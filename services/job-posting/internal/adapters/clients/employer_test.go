package clients_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const employerID = "3f2b8c1e-1111-4222-8333-444455556666"

// fakeEmployer answers GetCompanyProfile with a fixed status or error and
// records what it received.
type fakeEmployer struct {
	employerv1.UnimplementedEmployerServiceServer
	status employerv1.VerificationStatus
	err    error
	block  bool // wait until the call is cancelled

	mu  sync.Mutex
	req *employerv1.GetCompanyProfileRequest
	md  metadata.MD
}

func (f *fakeEmployer) GetCompanyProfile(ctx context.Context, req *employerv1.GetCompanyProfileRequest) (*employerv1.GetCompanyProfileResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.mu.Lock()
	f.req, f.md = req, md
	f.mu.Unlock()
	if f.block {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	return &employerv1.GetCompanyProfileResponse{Profile: &employerv1.CompanyProfile{
		EmployerId: req.GetEmployerId(), CompanyName: "Druk Tech", VerificationStatus: f.status,
	}}, nil
}

func startEmployer(t *testing.T, fake *fakeEmployer) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := grpc.NewServer()
	employerv1.RegisterEmployerServiceServer(s, fake)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)
	return l.Addr().String()
}

// newClient dials the way job-posting's main.go does: the shared public interceptor.
func newClient(t *testing.T, addr string) *clients.EmployerClient {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return clients.NewEmployerClient(employerv1.NewEmployerServiceClient(conn))
}

// asEmployer is the incoming context of a PublishJob request from the gateway.
func asEmployer(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return metadata.NewIncomingContext(ctx, metadata.Pairs("x-user-id", employerID, "x-user-role", "employer"))
}

func TestEmployerClient_IsVerified_MapsEmployerResponses(t *testing.T) {
	const (
		unspecified = employerv1.VerificationStatus_VERIFICATION_STATUS_UNSPECIFIED
		unverified  = employerv1.VerificationStatus_VERIFICATION_STATUS_UNVERIFIED
		verified    = employerv1.VerificationStatus_VERIFICATION_STATUS_VERIFIED
		rejected    = employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED
	)
	tests := []struct {
		name         string
		status       employerv1.VerificationStatus
		err          error
		wantVerified bool
		wantErr      bool
	}{
		{"VERIFIED is verified", verified, nil, true, false},
		{"UNVERIFIED is not verified", unverified, nil, false, false},
		{"REJECTED is not verified", rejected, nil, false, false},
		{"NOT_FOUND (no profile) is not verified", unspecified, status.Error(codes.NotFound, "company profile not found"), false, false},
		{"UNSPECIFIED is an error", unspecified, nil, false, true},
		{"unknown status is an error", employerv1.VerificationStatus(99), nil, false, true},
		{"UNAVAILABLE is an error", unspecified, status.Error(codes.Unavailable, "employer down"), false, true},
		{"INTERNAL is an error", unspecified, status.Error(codes.Internal, "boom"), false, true},
		{"UNAUTHENTICATED is an error", unspecified, status.Error(codes.Unauthenticated, "no identity"), false, true},
		{"PERMISSION_DENIED is an error", unspecified, status.Error(codes.PermissionDenied, "denied"), false, true},
		{"INVALID_ARGUMENT is an error", unspecified, status.Error(codes.InvalidArgument, "bad id"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEmployer{status: tt.status, err: tt.err}
			got, err := newClient(t, startEmployer(t, fake)).IsVerified(asEmployer(t), employerID)
			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrEmployerCheckFailed)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantVerified, got)

			// Every call asks about the right employer and forwards the caller.
			fake.mu.Lock()
			defer fake.mu.Unlock()
			require.NotNil(t, fake.req)
			assert.Equal(t, employerID, fake.req.GetEmployerId())
			assert.Equal(t, []string{employerID}, fake.md.Get("x-user-id"))
			assert.Equal(t, []string{"employer"}, fake.md.Get("x-user-role"))
		})
	}
}

func TestEmployerClient_IsVerified_EmployerDownIsAnError(t *testing.T) {
	// A closed port: the connection itself fails with UNAVAILABLE.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	got, err := newClient(t, addr).IsVerified(asEmployer(t), employerID)
	require.ErrorIs(t, err, domain.ErrEmployerCheckFailed)
	assert.False(t, got)
}

func TestEmployerClient_IsVerified_TimeoutIsAnError(t *testing.T) {
	fake := &fakeEmployer{block: true}
	ctx, cancel := context.WithTimeout(asEmployer(t), 100*time.Millisecond)
	defer cancel()

	got, err := newClient(t, startEmployer(t, fake)).IsVerified(ctx, employerID)
	require.ErrorIs(t, err, domain.ErrEmployerCheckFailed)
	assert.False(t, got)
}

func TestEmployerClient_IsVerified_ErrorHidesEmployerMessage(t *testing.T) {
	fake := &fakeEmployer{err: status.Error(codes.Internal, "pq: password authentication failed")}
	_, err := newClient(t, startEmployer(t, fake)).IsVerified(asEmployer(t), employerID)
	require.ErrorIs(t, err, domain.ErrEmployerCheckFailed)
	assert.NotContains(t, err.Error(), "password")
}
