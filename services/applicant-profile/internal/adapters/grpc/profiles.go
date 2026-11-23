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
// The applicant RPCs work on the caller's own profile; no request field names
// an applicant. The two JT-114 employer RPCs name an application instead, and
// the applicant comes from the local projection.
type ApplicantProfileHandler struct {
	applicantprofilev1.UnimplementedApplicantProfileServiceServer
	profiles     *app.Profiles
	applications *app.ApplicationParticipants
}

var _ applicantprofilev1.ApplicantProfileServiceServer = (*ApplicantProfileHandler)(nil)

func NewApplicantProfileHandler(profiles *app.Profiles, applications *app.ApplicationParticipants) *ApplicantProfileHandler {
	return &ApplicantProfileHandler{profiles: profiles, applications: applications}
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

// PutMyProfilePhoto stores or replaces the caller's photo (JT-103).
func (h *ApplicantProfileHandler) PutMyProfilePhoto(ctx context.Context, req *applicantprofilev1.PutMyProfilePhotoRequest) (*applicantprofilev1.PutMyProfilePhotoResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p, err := h.profiles.PutPhoto(ctx, caller, req.GetContentType(), req.GetContent())
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.PutMyProfilePhotoResponse{Profile: profileToProto(p)}, nil
}

// GetMyProfilePhoto returns the stored, already validated content type, so a
// client never receives a type that was not checked (JT-103).
func (h *ApplicantProfileHandler) GetMyProfilePhoto(ctx context.Context, _ *applicantprofilev1.GetMyProfilePhotoRequest) (*applicantprofilev1.GetMyProfilePhotoResponse, error) {
	caller, err := applicantFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	photo, err := h.profiles.GetPhoto(ctx, caller)
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.GetMyProfilePhotoResponse{ContentType: photo.ContentType, Content: photo.Content}, nil
}

// GetApplicantProfileForApplication is employer only (JT-114 Part C): the
// role is checked before the application_id is read. The profile never
// carries photo bytes (JT-103).
func (h *ApplicantProfileHandler) GetApplicantProfileForApplication(ctx context.Context, req *applicantprofilev1.GetApplicantProfileForApplicationRequest) (*applicantprofilev1.GetApplicantProfileForApplicationResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	p, err := h.applications.Profile(ctx, caller, req.GetApplicationId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.GetApplicantProfileForApplicationResponse{Profile: profileToProto(p)}, nil
}

// GetApplicantPhotoForApplication has the same access rule and returns the
// stored, already validated content type.
func (h *ApplicantProfileHandler) GetApplicantPhotoForApplication(ctx context.Context, req *applicantprofilev1.GetApplicantPhotoForApplicationRequest) (*applicantprofilev1.GetApplicantPhotoForApplicationResponse, error) {
	caller, err := employerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	photo, err := h.applications.Photo(ctx, caller, req.GetApplicationId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &applicantprofilev1.GetApplicantPhotoForApplicationResponse{ContentType: photo.ContentType, Content: photo.Content}, nil
}

// applicantFrom trusts x-user-id and x-user-role only because the gateway
// sets them and strips any client-sent values (JT-31). The role is checked
// before any input, so other roles learn nothing about the validation rules.
func applicantFrom(ctx context.Context) (domain.Caller, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireApplicant(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

// employerFrom is applicantFrom for the two JT-114 employer RPCs.
func employerFrom(ctx context.Context) (domain.Caller, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return domain.Caller{}, err
	}
	if err := caller.RequireEmployer(); err != nil {
		return domain.Caller{}, err
	}
	return caller, nil
}

func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
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
		HasPhoto: p.HasPhoto,
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
		errors.Is(err, domain.ErrDuplicateSkill), errors.Is(err, domain.ErrInvalidPhotoSize),
		errors.Is(err, domain.ErrInvalidPhotoType), errors.Is(err, domain.ErrInvalidPhotoContent),
		errors.Is(err, domain.ErrInvalidApplicationID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrApplicantRoleRequired), errors.Is(err, domain.ErrEmployerRoleRequired),
		errors.Is(err, domain.ErrNotApplicationEmployer):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrProfileNotFound), errors.Is(err, domain.ErrPhotoNotFound),
		errors.Is(err, domain.ErrApplicationNotFound):
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
