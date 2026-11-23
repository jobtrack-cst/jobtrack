package grpc_test

import (
	"context"
	"testing"

	apv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	adapter "github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/ports/mocks"
)

// newHandlerWithParticipants, withCaller, wantCode, storedProfile, errDB,
// applicantID and otherID are in profiles_test.go.

const (
	applicationID   = "44444444-4444-4444-8444-444444444444"
	owningEmployer  = "55555555-5555-4555-8555-555555555555"
	anotherEmployer = "66666666-6666-4666-8666-666666666666"
)

func storedParticipant() domain.ApplicationParticipant {
	return domain.ApplicationParticipant{ApplicationID: applicationID, ApplicantID: applicantID, EmployerID: owningEmployer}
}

func asOwningEmployer() context.Context { return withCaller(owningEmployer, "employer") }

// employerCalls are the two JT-114 RPCs, each called with one application_id.
var employerCalls = map[string]func(*adapter.ApplicantProfileHandler, context.Context, string) error{
	"GetApplicantProfileForApplication": func(h *adapter.ApplicantProfileHandler, c context.Context, id string) error {
		_, err := h.GetApplicantProfileForApplication(c, &apv1.GetApplicantProfileForApplicationRequest{ApplicationId: id})
		return err
	},
	"GetApplicantPhotoForApplication": func(h *adapter.ApplicantProfileHandler, c context.Context, id string) error {
		_, err := h.GetApplicantPhotoForApplication(c, &apv1.GetApplicantPhotoForApplicationRequest{ApplicationId: id})
		return err
	},
}

// Rows 1 to 4 of the JT-114 Section 4 rules table, the same for both RPCs.
func TestHandler_EmployerRPCs_AccessRules(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		id    string
		setup func(*mocks.MockApplicationParticipantRepository)
		want  codes.Code
	}{
		{name: "applicant is denied before input", ctx: withCaller(applicantID, "applicant"), id: "bad", want: codes.PermissionDenied},
		{name: "admin is denied before input", ctx: withCaller(otherID, "admin"), id: "bad", want: codes.PermissionDenied},
		{name: "application_id not a UUID", ctx: asOwningEmployer(), id: "bad", want: codes.InvalidArgument},
		{name: "empty request", ctx: asOwningEmployer(), id: "", want: codes.InvalidArgument},
		{
			name: "unknown or unprocessed application", ctx: asOwningEmployer(), id: applicationID, want: codes.NotFound,
			setup: func(p *mocks.MockApplicationParticipantRepository) {
				p.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationParticipant{}, domain.ErrApplicationNotFound)
			},
		},
		{
			name: "another employer", ctx: withCaller(anotherEmployer, "employer"), id: applicationID, want: codes.PermissionDenied,
			setup: func(p *mocks.MockApplicationParticipantRepository) {
				p.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
			},
		},
	} {
		for rpc, call := range employerCalls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				// No profile or photo read is expected: gomock fails the test if one happens.
				h, _, participants := newHandlerWithParticipants(t)
				if tt.setup != nil {
					tt.setup(participants)
				}
				wantCode(t, call(h, tt.ctx, tt.id), tt.want)
			})
		}
	}
}

func TestHandler_EmployerRPCs_RejectBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", owningEmployer))},
		{"user ID not a UUID", withCaller("abc", "employer")},
		{"uppercase role", withCaller(owningEmployer, "Employer")},
	} {
		for rpc, call := range employerCalls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, _, _ := newHandlerWithParticipants(t)
				wantCode(t, call(h, tt.ctx, applicationID), codes.Unauthenticated)
			})
		}
	}
}

func TestHandler_GetApplicantProfileForApplication(t *testing.T) {
	// Row 5: the applicant has no profile.
	t.Run("applicant has no profile", func(t *testing.T) {
		h, repo, participants := newHandlerWithParticipants(t)
		participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
		repo.EXPECT().Get(gomock.Any(), applicantID).Return(domain.Profile{}, domain.ErrProfileNotFound)
		resp, err := h.GetApplicantProfileForApplication(asOwningEmployer(), &apv1.GetApplicantProfileForApplicationRequest{ApplicationId: applicationID})
		wantCode(t, err, codes.NotFound)
		assert.Nil(t, resp)
	})
	// Row 6: the current profile; has_photo is reported, the bytes are not
	// (ApplicantProfile has no bytes field, and GetPhoto is never called).
	t.Run("owning employer reads the current profile", func(t *testing.T) {
		h, repo, participants := newHandlerWithParticipants(t)
		participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
		profile := storedProfile()
		profile.HasPhoto = true
		repo.EXPECT().Get(gomock.Any(), applicantID).Return(profile, nil)
		resp, err := h.GetApplicantProfileForApplication(asOwningEmployer(), &apv1.GetApplicantProfileForApplicationRequest{ApplicationId: applicationID})
		require.NoError(t, err)
		got := resp.GetProfile()
		assert.Equal(t, applicantID, got.GetApplicantId())
		assert.Equal(t, "Backend developer", got.GetSummary())
		assert.Equal(t, []string{"Go", "SQL"}, got.GetSkills())
		require.Len(t, got.GetEducation(), 2)
		assert.Equal(t, "RUB", got.GetEducation()[0].GetInstitution())
		assert.True(t, got.GetHasPhoto())
		assert.Equal(t, profile.UpdatedAt, got.GetTimestamps().GetUpdatedAt().AsTime())
	})
}

func TestHandler_GetApplicantPhotoForApplication(t *testing.T) {
	// Row 5: no profile or no photo.
	t.Run("applicant has no photo", func(t *testing.T) {
		h, repo, participants := newHandlerWithParticipants(t)
		participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, domain.ErrPhotoNotFound)
		resp, err := h.GetApplicantPhotoForApplication(asOwningEmployer(), &apv1.GetApplicantPhotoForApplicationRequest{ApplicationId: applicationID})
		wantCode(t, err, codes.NotFound)
		assert.Nil(t, resp)
	})
	// Row 6: the current photo with its stored type.
	t.Run("owning employer reads the current photo", func(t *testing.T) {
		h, repo, participants := newHandlerWithParticipants(t)
		participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(storedParticipant(), nil)
		photo := domain.Photo{ContentType: "image/jpeg", Content: []byte{0xff, 0xd8, 0xff, 0x01}}
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(photo, nil)
		resp, err := h.GetApplicantPhotoForApplication(asOwningEmployer(), &apv1.GetApplicantPhotoForApplicationRequest{ApplicationId: applicationID})
		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", resp.GetContentType())
		assert.Equal(t, photo.Content, resp.GetContent())
	})
}

func TestHandler_EmployerRPCs_ErrorsDoNotLeak(t *testing.T) {
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
		for rpc, call := range employerCalls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, _, participants := newHandlerWithParticipants(t)
				participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(domain.ApplicationParticipant{}, tt.repoErr)
				err := call(h, asOwningEmployer(), applicationID)
				wantCode(t, err, tt.wantCode)
				require.Equal(t, tt.wantMsg, status.Convert(err).Message())
			})
		}
	}
}
