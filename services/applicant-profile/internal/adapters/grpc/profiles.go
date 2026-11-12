package grpc

import (
	"context"
	"errors"

	applicantprofilev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// ApplicantProfileHandler serves ApplicantProfileService through the gateway.
// Every RPC works on the caller's own profile; no request field names an
// applicant.
type ApplicantProfileHandler struct {
	applicantprofilev1.UnimplementedApplicantProfileServiceServer
	profiles *app.Profiles
}

var _ applicantprofilev1.ApplicantProfileServiceServer = (*ApplicantProfileHandler)(nil)

func NewApplicantProfileHandler(profiles *app.Profiles) *ApplicantProfileHandler {
	return &ApplicantProfileHandler{profiles: profiles}
}

// The generated getters are nil-safe, so a nil request is an empty one.
func (h *ApplicantProfileHandler) CreateMyProfile(ctx context.Context, req *applicantprofilev1.CreateMyProfileRequest) (*applicantprofilev1.CreateMyProfileResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p, err := h.profiles.Create(ctx, caller, input(req.GetSummary(), req.GetEducation(), req.GetSkills()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.CreateMyProfileResponse{Profile: profileToProto(p)}, nil
}

func (h *ApplicantProfileHandler) GetMyProfile(ctx context.Context, _ *applicantprofilev1.GetMyProfileRequest) (*applicantprofilev1.GetMyProfileResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p, err := h.profiles.Get(ctx, caller)
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.GetMyProfileResponse{Profile: profileToProto(p)}, nil
}

// UpdateMyProfile replaces all three fields (JT-50): an empty list clears it.
func (h *ApplicantProfileHandler) UpdateMyProfile(ctx context.Context, req *applicantprofilev1.UpdateMyProfileRequest) (*applicantprofilev1.UpdateMyProfileResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p, err := h.profiles.Update(ctx, caller, input(req.GetSummary(), req.GetEducation(), req.GetSkills()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.UpdateMyProfileResponse{Profile: profileToProto(p)}, nil
}

// applicantFrom trusts x-user-id and x-user-role only because the gateway
// sets them and strips any client-sent values (JT-31). The role is checked
// before any input, so other roles learn nothing about the validation rules.
func applicantFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	caller, err := domain.NewCaller(identity.UserID, identity.Role)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireApplicant(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

func input(summary string, education []*applicantprofilev1.Education, skills []string) domain.ProfileInput {
	in := domain.ProfileInput{Summary: summary, Skills: skills, Education: make([]domain.Education, 0, len(education))}
	for _, e := range education {
		// A nil entry reads as empty and fails validation.
		in.Education = append(in.Education, domain.Education{
			Institution: e.GetInstitution(), Qualification: e.GetQualification(),
			StartYear: e.GetStartYear(), EndYear: e.GetEndYear(),
		})
	}
	return in
}

func profileToProto(p domain.Profile) *applicantprofilev1.ApplicantProfile {
	education := make([]*applicantprofilev1.Education, 0, len(p.Education))
	for _, e := range p.Education {
		education = append(education, &applicantprofilev1.Education{
			Institution: e.Institution, Qualification: e.Qualification, StartYear: e.StartYear, EndYear: e.EndYear,
		})
	}
	return &applicantprofilev1.ApplicantProfile{
		ApplicantId: p.ApplicantID,
		Summary:     p.Summary,
		Education:   education,
		Skills:      append([]string{}, p.Skills...),
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
	case errors.Is(err, domain.ErrInvalidSummary), errors.Is(err, domain.ErrTooManyEducation),
		errors.Is(err, domain.ErrInvalidInstitution), errors.Is(err, domain.ErrInvalidQualification),
		errors.Is(err, domain.ErrInvalidStartYear), errors.Is(err, domain.ErrInvalidEndYear),
		errors.Is(err, domain.ErrTooManySkills), errors.Is(err, domain.ErrInvalidSkill),
		errors.Is(err, domain.ErrDuplicateSkill):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrApplicantRoleRequired):
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
