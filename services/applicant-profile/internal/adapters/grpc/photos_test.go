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
)

// newHandler, withCaller, asApplicant, wantCode, storedProfile, errDB and now
// are in profiles_test.go.

func photoBytes(signature []byte, n int) []byte {
	b := make([]byte, n)
	copy(b, signature)
	return b
}

var (
	jpegHeader = []byte{0xff, 0xd8, 0xff}
	pngHeader  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

var photoCalls = map[string]func(*adapter.ApplicantProfileHandler, context.Context) error{
	"PutMyProfilePhoto": func(h *adapter.ApplicantProfileHandler, c context.Context) error {
		_, err := h.PutMyProfilePhoto(c, &apv1.PutMyProfilePhotoRequest{ContentType: "image/jpeg", Content: photoBytes(jpegHeader, 10)})
		return err
	},
	"GetMyProfilePhoto": func(h *adapter.ApplicantProfileHandler, c context.Context) error {
		_, err := h.GetMyProfilePhoto(c, &apv1.GetMyProfilePhotoRequest{})
		return err
	},
}

func TestHandler_Photo_RejectsBadIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
	}{
		{"no metadata", context.Background()},
		{"missing role", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", applicantID))},
		{"user ID not a UUID", withCaller("abc", "applicant")},
		{"unknown role", withCaller(applicantID, "superuser")},
	} {
		for rpc, call := range photoCalls {
			t.Run(tt.name+"/"+rpc, func(t *testing.T) {
				h, _ := newHandler(t)
				wantCode(t, call(h, tt.ctx), codes.Unauthenticated)
			})
		}
	}
}

// Only the applicant uploads or sees their own photo; nothing is read or stored for other roles.
func TestHandler_Photo_OtherRolesDenied(t *testing.T) {
	for _, role := range []string{"employer", "admin"} {
		for rpc, call := range photoCalls {
			t.Run(role+"/"+rpc, func(t *testing.T) {
				h, _ := newHandler(t)
				wantCode(t, call(h, withCaller(otherID, role)), codes.PermissionDenied)
			})
		}
	}
	t.Run("role checked before input", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.PutMyProfilePhoto(withCaller(otherID, "employer"), &apv1.PutMyProfilePhotoRequest{ContentType: "image/gif"})
		wantCode(t, err, codes.PermissionDenied)
	})
}

func TestHandler_PutMyProfilePhoto_StoresForCallerAndReturnsHasPhoto(t *testing.T) {
	h, repo := newHandler(t)
	content := photoBytes(pngHeader, 1_000_000)
	withPhoto := storedProfile()
	withPhoto.HasPhoto = true
	// The owner comes from metadata, never the request.
	repo.EXPECT().PutPhoto(gomock.Any(), applicantID, domain.Photo{ContentType: "image/png", Content: content}, now).
		Return(withPhoto, nil)
	resp, err := h.PutMyProfilePhoto(asApplicant(), &apv1.PutMyProfilePhotoRequest{ContentType: "image/png", Content: content})
	require.NoError(t, err)
	assert.True(t, resp.GetProfile().GetHasPhoto())
	assert.Equal(t, applicantID, resp.GetProfile().GetApplicantId())
}

func TestHandler_PutMyProfilePhoto_InvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		req  *apv1.PutMyProfilePhotoRequest
	}{
		{"nil request", nil},
		{"0 bytes", &apv1.PutMyProfilePhotoRequest{ContentType: "image/jpeg"}},
		{"1 byte", &apv1.PutMyProfilePhotoRequest{ContentType: "image/jpeg", Content: []byte{0xff}}},
		{"1,000,001 bytes", &apv1.PutMyProfilePhotoRequest{ContentType: "image/jpeg", Content: photoBytes(jpegHeader, 1_000_001)}},
		{"wrong type", &apv1.PutMyProfilePhotoRequest{ContentType: "image/gif", Content: photoBytes(jpegHeader, 10)}},
		{"wrong signature", &apv1.PutMyProfilePhotoRequest{ContentType: "image/png", Content: photoBytes(jpegHeader, 10)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.PutMyProfilePhoto(asApplicant(), tt.req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestHandler_PutMyProfilePhoto_NoProfile(t *testing.T) {
	h, repo := newHandler(t)
	repo.EXPECT().PutPhoto(gomock.Any(), applicantID, gomock.Any(), now).Return(domain.Profile{}, domain.ErrProfileNotFound)
	_, err := h.PutMyProfilePhoto(asApplicant(), &apv1.PutMyProfilePhotoRequest{ContentType: "image/jpeg", Content: photoBytes(jpegHeader, 10)})
	wantCode(t, err, codes.NotFound)
}

func TestHandler_GetMyProfilePhoto(t *testing.T) {
	t.Run("returns the stored type and bytes", func(t *testing.T) {
		h, repo := newHandler(t)
		content := photoBytes(jpegHeader, 50)
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{ContentType: "image/jpeg", Content: content}, nil)
		resp, err := h.GetMyProfilePhoto(asApplicant(), nil)
		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", resp.GetContentType())
		assert.Equal(t, content, resp.GetContent())
	})
	t.Run("no photo is NOT_FOUND", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, domain.ErrPhotoNotFound)
		_, err := h.GetMyProfilePhoto(asApplicant(), nil)
		wantCode(t, err, codes.NotFound)
	})
	t.Run("database error is INTERNAL without details", func(t *testing.T) {
		h, repo := newHandler(t)
		repo.EXPECT().GetPhoto(gomock.Any(), applicantID).Return(domain.Photo{}, errDB)
		_, err := h.GetMyProfilePhoto(asApplicant(), nil)
		wantCode(t, err, codes.Internal)
		assert.NotContains(t, status.Convert(err).Message(), "SQL")
	})
}

// Profile reads report whether a photo exists without the bytes.
func TestHandler_GetMyProfile_ReportsHasPhoto(t *testing.T) {
	h, repo := newHandler(t)
	p := storedProfile()
	p.HasPhoto = true
	repo.EXPECT().Get(gomock.Any(), applicantID).Return(p, nil)
	resp, err := h.GetMyProfile(asApplicant(), nil)
	require.NoError(t, err)
	assert.True(t, resp.GetProfile().GetHasPhoto())
}
