package grpc

import (
	"context"
	"errors"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/employer/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EmployerHandler serves the public EmployerService through the gateway, and
// GetCompanyProfile for job-posting with the user's identity forwarded.
type EmployerHandler struct {
	employerv1.UnimplementedEmployerServiceServer
	profiles *app.Profiles
}

var _ employerv1.EmployerServiceServer = (*EmployerHandler)(nil)

func NewEmployerHandler(profiles *app.Profiles) *EmployerHandler {
	return &EmployerHandler{profiles: profiles}
}

func (h *EmployerHandler) CreateCompanyProfile(ctx context.Context, req *employerv1.CreateCompanyProfileRequest) (*employerv1.CreateCompanyProfileResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	profile, err := h.profiles.Create(ctx, caller, app.ProfileInput{
		CompanyName: req.GetCompanyName(), Description: req.GetDescription(),
		Website: req.GetWebsite(), Location: req.GetLocation(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &employerv1.CreateCompanyProfileResponse{Profile: profileToProto(profile)}, nil
}

func (h *EmployerHandler) GetMyCompanyProfile(ctx context.Context, _ *employerv1.GetMyCompanyProfileRequest) (*employerv1.GetMyCompanyProfileResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	profile, err := h.profiles.GetMine(ctx, caller)
	if err != nil {
		return nil, toStatus(err)
	}
	return &employerv1.GetMyCompanyProfileResponse{Profile: profileToProto(profile)}, nil
}

func (h *EmployerHandler) UpdateCompanyProfile(ctx context.Context, req *employerv1.UpdateCompanyProfileRequest) (*employerv1.UpdateCompanyProfileResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	profile, err := h.profiles.Update(ctx, caller, app.ProfileInput{
		CompanyName: req.GetCompanyName(), Description: req.GetDescription(),
		Website: req.GetWebsite(), Location: req.GetLocation(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &employerv1.UpdateCompanyProfileResponse{Profile: profileToProto(profile)}, nil
}

func (h *EmployerHandler) GetCompanyProfile(ctx context.Context, req *employerv1.GetCompanyProfileRequest) (*employerv1.GetCompanyProfileResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	profile, err := h.profiles.Get(ctx, caller, req.GetEmployerId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &employerv1.GetCompanyProfileResponse{Profile: profileToProto(profile)}, nil
}

// callerFrom trusts x-user-id and x-user-role only because the gateway sets
// them (or job-posting forwards them through pkg/grpcclient).
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

func statusToProto(s domain.VerificationStatus) employerv1.VerificationStatus {
	switch s {
	case domain.VerificationUnverified:
		return employerv1.VerificationStatus_VERIFICATION_STATUS_UNVERIFIED
	case domain.VerificationVerified:
		return employerv1.VerificationStatus_VERIFICATION_STATUS_VERIFIED
	case domain.VerificationRejected:
		return employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED
	default:
		return employerv1.VerificationStatus_VERIFICATION_STATUS_UNSPECIFIED
	}
}

// profileToProto expects a profile already filtered by domain ViewFor.
func profileToProto(p domain.CompanyProfile) *employerv1.CompanyProfile {
	return &employerv1.CompanyProfile{
		EmployerId:         p.EmployerID,
		CompanyName:        p.CompanyName,
		Description:        p.Description,
		Website:            p.Website,
		Location:           p.Location,
		VerificationStatus: statusToProto(p.VerificationStatus),
		RejectionReason:    p.RejectionReason,
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(p.CreatedAt),
			UpdatedAt: timestamppb.New(p.UpdatedAt),
		},
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidEmployerID), errors.Is(err, domain.ErrInvalidCompanyName),
		errors.Is(err, domain.ErrInvalidDescription), errors.Is(err, domain.ErrInvalidWebsite),
		errors.Is(err, domain.ErrInvalidLocation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrEmployerRoleRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrProfileNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrProfileExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
