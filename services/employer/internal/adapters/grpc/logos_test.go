package grpc_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
)

// Helpers and fixtures (newHandler, withCaller, wantCode, rejected, testTime,
// errDB, identityCases) are in employer_test.go.

func grpcLogo(signature []byte, n int) []byte {
	b := make([]byte, n)
	copy(b, signature)
	return b
}

var (
	pngBytes  = grpcLogo([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 64)
	jpegBytes = grpcLogo([]byte{0xff, 0xd8, 0xff}, 64)
	putReq    = &employerv1.PutCompanyLogoRequest{ContentType: "image/png", Content: pngBytes}
)

func TestEmployerHandler_LogoRPCsRejectBadIdentity(t *testing.T) {
	for _, tt := range identityCases {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newHandler(t)
			_, err := h.PutCompanyLogo(tt.ctx, putReq)
			wantCode(t, err, tt.want)
			_, err = h.GetCompanyLogo(tt.ctx, &employerv1.GetCompanyLogoRequest{EmployerId: employerID})
			wantCode(t, err, tt.want)
		})
	}
}

func TestEmployerHandler_PutCompanyLogo(t *testing.T) {
	later := testTime.Add(time.Hour)
	withLogo := rejected
	withLogo.HasLogo, withLogo.UpdatedAt = true, later
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *employerv1.PutCompanyLogoRequest
		setup func(d deps)
		want  codes.Code
	}{
		{
			name: "owner uploads and verification is unchanged", ctx: withCaller(employerID, "employer"), req: putReq, want: codes.OK,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(later)
				d.repo.EXPECT().PutLogo(gomock.Any(), employerID, domain.Logo{ContentType: "image/png", Content: pngBytes}, later).
					Return(withLogo, nil)
			},
		},
		{name: "applicant is denied", ctx: withCaller(employerID, "applicant"), req: putReq, want: codes.PermissionDenied},
		{name: "admin is denied", ctx: withCaller(employerID, "admin"), req: putReq, want: codes.PermissionDenied},
		{name: "role is checked before input", ctx: withCaller(employerID, "applicant"),
			req: &employerv1.PutCompanyLogoRequest{ContentType: "image/gif"}, want: codes.PermissionDenied},
		{name: "nil request", ctx: withCaller(employerID, "employer"), want: codes.InvalidArgument},
		{name: "0 bytes", ctx: withCaller(employerID, "employer"),
			req: &employerv1.PutCompanyLogoRequest{ContentType: "image/png"}, want: codes.InvalidArgument},
		{name: "1,000,001 bytes", ctx: withCaller(employerID, "employer"),
			req: &employerv1.PutCompanyLogoRequest{ContentType: "image/jpeg", Content: grpcLogo([]byte{0xff, 0xd8, 0xff}, 1_000_001)}, want: codes.InvalidArgument},
		{name: "wrong type", ctx: withCaller(employerID, "employer"),
			req: &employerv1.PutCompanyLogoRequest{ContentType: "image/gif", Content: pngBytes}, want: codes.InvalidArgument},
		{name: "wrong signature", ctx: withCaller(employerID, "employer"),
			req: &employerv1.PutCompanyLogoRequest{ContentType: "image/png", Content: jpegBytes}, want: codes.InvalidArgument},
		{
			name: "no profile yet", ctx: withCaller(employerID, "employer"), req: putReq, want: codes.NotFound,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(later)
				d.repo.EXPECT().PutLogo(gomock.Any(), employerID, gomock.Any(), later).Return(domain.CompanyProfile{}, domain.ErrProfileNotFound)
			},
		},
		{
			name: "database failure is hidden", ctx: withCaller(employerID, "employer"), req: putReq, want: codes.Internal,
			setup: func(d deps) {
				d.clock.EXPECT().Now().Return(later)
				d.repo.EXPECT().PutLogo(gomock.Any(), employerID, gomock.Any(), later).Return(domain.CompanyProfile{}, errDB)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.PutCompanyLogo(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("driver detail leaked: %v", err)
			}
			if tt.want != codes.OK {
				return
			}
			p := resp.GetProfile()
			if !p.GetHasLogo() || p.GetEmployerId() != employerID || p.GetRejectionReason() != reason ||
				p.GetVerificationStatus() != employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED ||
				!p.GetTimestamps().GetUpdatedAt().AsTime().Equal(later) {
				t.Fatalf("unexpected profile %v", p)
			}
		})
	}
}

func TestEmployerHandler_GetCompanyLogo(t *testing.T) {
	stored := domain.Logo{ContentType: "image/jpeg", Content: jpegBytes}
	found := func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(stored, nil) }
	for _, tt := range []struct {
		name  string
		ctx   context.Context
		req   *employerv1.GetCompanyLogoRequest
		setup func(d deps)
		want  codes.Code
	}{
		{name: "applicant reads logo", ctx: withCaller(otherID, "applicant"), req: &employerv1.GetCompanyLogoRequest{EmployerId: employerID}, setup: found, want: codes.OK},
		{name: "admin reads logo", ctx: withCaller(otherID, "admin"), req: &employerv1.GetCompanyLogoRequest{EmployerId: employerID}, setup: found, want: codes.OK},
		{name: "owner reads logo", ctx: withCaller(employerID, "employer"), req: &employerv1.GetCompanyLogoRequest{EmployerId: employerID}, setup: found, want: codes.OK},
		{name: "ID not a UUID", ctx: withCaller(otherID, "applicant"), req: &employerv1.GetCompanyLogoRequest{EmployerId: "abc"}, want: codes.InvalidArgument},
		{name: "nil request", ctx: withCaller(otherID, "applicant"), want: codes.InvalidArgument},
		{
			name: "no logo", ctx: withCaller(otherID, "applicant"), req: &employerv1.GetCompanyLogoRequest{EmployerId: otherID}, want: codes.NotFound,
			setup: func(d deps) {
				d.repo.EXPECT().GetLogo(gomock.Any(), otherID).Return(domain.Logo{}, domain.ErrLogoNotFound)
			},
		},
		{
			name: "database failure is hidden", ctx: withCaller(otherID, "applicant"), req: &employerv1.GetCompanyLogoRequest{EmployerId: employerID}, want: codes.Internal,
			setup: func(d deps) { d.repo.EXPECT().GetLogo(gomock.Any(), employerID).Return(domain.Logo{}, errDB) },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, d := newHandler(t)
			if tt.setup != nil {
				tt.setup(d)
			}
			resp, err := h.GetCompanyLogo(tt.ctx, tt.req)
			wantCode(t, err, tt.want)
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("driver detail leaked: %v", err)
			}
			if tt.want != codes.OK {
				return
			}
			if resp.GetContentType() != "image/jpeg" || !bytes.Equal(resp.GetContent(), jpegBytes) {
				t.Fatalf("got %q with %d bytes, want the stored JPEG", resp.GetContentType(), len(resp.GetContent()))
			}
		})
	}
}

// Profile responses carry has_logo but never image bytes; the message has no
// field for them, and the flag is copied from the stored profile.
func TestEmployerHandler_GetCompanyProfile_ReportsHasLogo(t *testing.T) {
	withLogo := rejected
	withLogo.HasLogo = true
	h, d := newHandler(t)
	d.repo.EXPECT().GetByEmployerID(gomock.Any(), employerID).Return(withLogo, nil)
	resp, err := h.GetCompanyProfile(withCaller(otherID, "applicant"), &employerv1.GetCompanyProfileRequest{EmployerId: employerID})
	wantCode(t, err, codes.OK)
	if !resp.GetProfile().GetHasLogo() {
		t.Fatalf("has_logo=false, want true")
	}
}
