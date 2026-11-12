package grpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports/mocks"
)

const (
	applicantID = "11111111-1111-4111-8111-111111111111"
	otherID     = "22222222-2222-4222-8222-222222222222"
)

var (
	errDB = errors.New("pq: secret SQL detail")
	now   = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
)

// Handlers are tested with the real app layer and a mocked repository. Any
// repository call a case does not expect fails the test.
func newHandler(t *testing.T) (*adapter.ApplicantProfileHandler, *mocks.MockProfileRepository) {
	ctrl := gomock.NewController(t)
	clock := mocks.NewMockClock(ctrl)
	clock.EXPECT().Now().Return(now).AnyTimes()
	repo := mocks.NewMockProfileRepository(ctrl)
	return adapter.NewApplicantProfileHandler(app.NewProfiles(repo, clock)), repo
}

func withCaller(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func asApplicant() context.Context { return withCaller(applicantID, "applicant") }

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v (%v), want %v", got, err, want)
	}
}

func protoEducation() []*apv1.Education {
	return []*apv1.Education{
		{Institution: " RUB ", Qualification: "BSc", StartYear: 2019, EndYear: 2022},
		{Institution: "CST", Qualification: "BE", StartYear: 2023},
	}
}

func storedProfile() domain.Profile {
	return domain.Profile{
		ApplicantID: applicantID,
		ProfileContent: domain.ProfileContent{
			Summary: "Backend developer",
			Education: []domain.Education{
				{Institution: "RUB", Qualification: "BSc", StartYear: 2019, EndYear: 2022},
				{Institution: "CST", Qualification: "BE", StartYear: 2023},
			},
			Skills: []string{"Go", "SQL"},
		},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
}

var calls = map[string]func(*adapter.ApplicantProfileHandler, context.Context) error{
	"CreateMyProfile": func(h *adapter.ApplicantProfileHandler, c context.Context) error {
		_, err := h.CreateMyProfile(c, &apv1.CreateMyProfileRequest{Summary: "x"})
		return err
	},
	"GetMyProfile": func(h *adapter.ApplicantProfileHandler, c context.Context) error {
		_, err := h.GetMyProfile(c, &apv1.GetMyProfileRequest{})
		return err
	},
	"UpdateMyProfile": func(h *adapter.ApplicantProfileHandler, c context.Context) error {
		_, err := h.UpdateMyProfile(c, &apv1.UpdateMyProfileRequest{Summary: "x"})
		return err
	},
}

func TestHandler_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", applicantID))},
		{"missing user ID", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-role", "applicant"))},
		{"user ID not a UUID", withCaller("abc", "applicant")},
		{"unknown role", withCaller(applicantID, "superuser")},
		{"uppercase role", withCaller(applicantID, "Applicant")},
	} {
		for rpc, call := range calls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, _ := newHandler(t)
				wantCode(t, call(h, tt.ctx), codes.Unauthenticated)
			})
		}
	}
}

// Every RPC is applicant only; the role is checked before input or storage.
func TestHandler_OtherRolesDenied(t *testing.T) {
	for _, role := range []string{"employer", "admin"} {
		for rpc, call := range calls {
			t.Run(role+"/"+rpc, func(t *testing.T) {
				h, _ := newHandler(t)
				wantCode(t, call(h, withCaller(otherID, role)), codes.PermissionDenied)
			})
		}
	}
	h, _ := newHandler(t)
	_, err := h.CreateMyProfile(withCaller(otherID, "employer"), &apv1.CreateMyProfileRequest{Skills: []string{""}})
	wantCode(t, err, codes.PermissionDenied)
}

func TestHandler_CreateMyProfile_MapsRequestAndResponse(t *testing.T) {
	h, repo := newHandler(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p domain.Profile) error {
		assert.Equal(t, applicantID, p.ApplicantID, "owner comes from metadata, never the request")
		assert.Equal(t, "RUB", p.Education[0].Institution, "trimmed")
		assert.Equal(t, []string{"Go", "SQL"}, p.Skills)
		return nil
	})
	resp, err := h.CreateMyProfile(asApplicant(), &apv1.CreateMyProfileRequest{
		Summary: " Backend developer ", Education: protoEducation(), Skills: []string{"Go", "SQL"},
	})
	require.NoError(t, err)
	p := resp.GetProfile()
	assert.Equal(t, applicantID, p.GetApplicantId())
	assert.Equal(t, "Backend developer", p.GetSummary())
	require.Len(t, p.GetEducation(), 2)
	assert.Equal(t, "RUB", p.GetEducation()[0].GetInstitution())
	assert.Equal(t, int32(2022), p.GetEducation()[0].GetEndYear())
	assert.Equal(t, int32(0), p.GetEducation()[1].GetEndYear(), "ongoing")
	assert.Equal(t, []string{"Go", "SQL"}, p.GetSkills())
	assert.Equal(t, now, p.GetTimestamps().GetCreatedAt().AsTime())
	assert.Equal(t, now, p.GetTimestamps().GetUpdatedAt().AsTime())
}

func TestHandler_InvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		req  *apv1.CreateMyProfileRequest
	}{
		{"summary 2001", &apv1.CreateMyProfileRequest{Summary: strings.Repeat("a", 2001)}},
		{"21 education entries", &apv1.CreateMyProfileRequest{Education: make([]*apv1.Education, 21)}},
		{"nil education entry", &apv1.CreateMyProfileRequest{Education: []*apv1.Education{nil}}},
		{"start year 1949", &apv1.CreateMyProfileRequest{Education: []*apv1.Education{{Institution: "A", Qualification: "B", StartYear: 1949}}}},
		{"end before start", &apv1.CreateMyProfileRequest{Education: []*apv1.Education{{Institution: "A", Qualification: "B", StartYear: 2020, EndYear: 2019}}}},
		{"51 skills", &apv1.CreateMyProfileRequest{Skills: make([]string, 51)}},
		{"duplicate skill", &apv1.CreateMyProfileRequest{Skills: []string{"Go", "GO"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.CreateMyProfile(asApplicant(), tt.req)
			wantCode(t, err, codes.InvalidArgument)
			_, err = h.UpdateMyProfile(asApplicant(), &apv1.UpdateMyProfileRequest{
				Summary: tt.req.GetSummary(), Education: tt.req.GetEducation(), Skills: tt.req.GetSkills(),
			})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
	t.Run("message names the bad entry", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.CreateMyProfile(asApplicant(), &apv1.CreateMyProfileRequest{Skills: []string{"Go", "Rust", "go"}})
		assert.Contains(t, status.Convert(err).Message(), "skills[2]")
	})
}

func TestHandler_CreateMyProfile_Twice(t *testing.T) {
	h, repo := newHandler(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrProfileExists)
	_, err := h.CreateMyProfile(asApplicant(), nil)
	wantCode(t, err, codes.AlreadyExists)
}

func TestHandler_GetMyProfile(t *testing.T) {
	t.Run("returns the caller's own profile", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), applicantID).Return(storedProfile(), nil)
		resp, err := h.GetMyProfile(asApplicant(), nil)
		require.NoError(t, err)
		assert.Equal(t, "Backend developer", resp.GetProfile().GetSummary())
		assert.Len(t, resp.GetProfile().GetEducation(), 2)
		assert.Equal(t, now.Add(-time.Hour), resp.GetProfile().GetTimestamps().GetCreatedAt().AsTime())
	})
	t.Run("no profile yet", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{}, domain.ErrProfileNotFound)
		_, err := h.GetMyProfile(asApplicant(), nil)
		wantCode(t, err, codes.NotFound)
	})
	t.Run("empty lists are returned as empty", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{ApplicantID: applicantID,
			ProfileContent: domain.ProfileContent{Education: []domain.Education{}, Skills: []string{}}}, nil)
		resp, err := h.GetMyProfile(asApplicant(), nil)
		require.NoError(t, err)
		assert.Empty(t, resp.GetProfile().GetEducation())
		assert.Empty(t, resp.GetProfile().GetSkills())
	})
}

func TestHandler_UpdateMyProfile(t *testing.T) {
	t.Run("replaces the caller's own profile", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Replace(gomock.Any(), applicantID, gomock.Any(), now).DoAndReturn(
			func(_ context.Context, _ string, c domain.ProfileContent, _ time.Time) (domain.Profile, error) {
				assert.Equal(t, []string{"Rust"}, c.Skills)
				assert.Empty(t, c.Education, "omitted list clears it")
				return domain.Profile{ApplicantID: applicantID, ProfileContent: c, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, nil
			})
		resp, err := h.UpdateMyProfile(asApplicant(), &apv1.UpdateMyProfileRequest{Skills: []string{"Rust"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"Rust"}, resp.GetProfile().GetSkills())
		assert.Equal(t, now.Add(-time.Hour), resp.GetProfile().GetTimestamps().GetCreatedAt().AsTime(), "created_at kept")
	})
	t.Run("no profile yet", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().Replace(gomock.Any(), applicantID, gomock.Any(), gomock.Any()).Return(domain.Profile{}, domain.ErrProfileNotFound)
		_, err := h.UpdateMyProfile(asApplicant(), nil)
		wantCode(t, err, codes.NotFound)
	})
}

func TestHandler_ErrorsDoNotLeak(t *testing.T) {
	for _, tt := range []struct {
		name     string
		repoErr  error
		wantCode codes.Code
		wantMsg  string
	}{
		{"database error", errDB, codes.Internal, "internal error"},
		{"canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := newHandler(t)
			repo.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{}, tt.repoErr)
			_, err := h.GetMyProfile(asApplicant(), nil)
			wantCode(t, err, tt.wantCode)
			assert.Equal(t, tt.wantMsg, status.Convert(err).Message())
		})
	}
}
