package grpc

import (
	"context"
	"errors"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthHandler serves the public AuthService through the gateway.
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	auth *app.Auth
}

var _ authv1.AuthServiceServer = (*AuthHandler)(nil)

func NewAuthHandler(auth *app.Auth) *AuthHandler { return &AuthHandler{auth: auth} }

func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := h.auth.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetFullName(), roleFromProto(req.GetRole()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.RegisterResponse{User: userToProto(user)}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	result, err := h.auth.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.LoginResponse{
		AccessToken: result.AccessToken,
		ExpiresAt:   timestamppb.New(result.ExpiresAt),
		User:        userToProto(result.User),
	}, nil
}

// GetCurrentUser trusts x-user-id and x-user-role only because the gateway sets them.
func (h *AuthHandler) GetCurrentUser(ctx context.Context, _ *authv1.GetCurrentUserRequest) (*authv1.GetCurrentUserResponse, error) {
	caller, ok := authctx.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "caller identity is missing")
	}
	if _, ok := domain.ParseRole(caller.Role); !ok {
		return nil, status.Error(codes.Unauthenticated, "caller role is not valid")
	}
	user, err := h.auth.GetCurrentUser(ctx, caller.UserID)
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.GetCurrentUserResponse{User: userToProto(user)}, nil
}

// InternalHandler serves AuthInternalService. The gateway never registers it.
// The x-internal-token interceptor is added in main.go once JT-44 is merged.
type InternalHandler struct {
	authv1.UnimplementedAuthInternalServiceServer
	auth *app.Auth
}

var _ authv1.AuthInternalServiceServer = (*InternalHandler)(nil)

func NewInternalHandler(auth *app.Auth) *InternalHandler { return &InternalHandler{auth: auth} }

func (h *InternalHandler) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	user, err := h.auth.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &authv1.GetUserResponse{User: userToProto(user)}, nil
}

func (h *InternalHandler) ListUsersByRole(ctx context.Context, req *authv1.ListUsersByRoleRequest) (*authv1.ListUsersByRoleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	page, err := h.auth.ListUsersByRole(ctx, roleFromProto(req.GetRole()),
		req.GetPagination().GetPageSize(), req.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	users := make([]*authv1.User, 0, len(page.Users))
	for _, u := range page.Users {
		users = append(users, userToProto(u))
	}
	return &authv1.ListUsersByRoleResponse{
		Users:      users,
		Pagination: &commonv1.PaginationResponse{NextPageToken: page.NextPageToken, TotalCount: page.TotalCount},
	}, nil
}

func roleFromProto(r authv1.Role) domain.Role {
	switch r {
	case authv1.Role_ROLE_APPLICANT:
		return domain.RoleApplicant
	case authv1.Role_ROLE_EMPLOYER:
		return domain.RoleEmployer
	case authv1.Role_ROLE_ADMIN:
		return domain.RoleAdmin
	default:
		return domain.RoleUnspecified
	}
}

func roleToProto(r domain.Role) authv1.Role {
	switch r {
	case domain.RoleApplicant:
		return authv1.Role_ROLE_APPLICANT
	case domain.RoleEmployer:
		return authv1.Role_ROLE_EMPLOYER
	case domain.RoleAdmin:
		return authv1.Role_ROLE_ADMIN
	default:
		return authv1.Role_ROLE_UNSPECIFIED
	}
}

// userToProto never copies PasswordHash.
func userToProto(u domain.User) *authv1.User {
	return &authv1.User{
		UserId:   u.ID,
		Email:    u.Email,
		FullName: u.FullName,
		Role:     roleToProto(u.Role),
		Timestamps: &commonv1.ResourceTimestamps{
			CreatedAt: timestamppb.New(u.CreatedAt),
			UpdatedAt: timestamppb.New(u.UpdatedAt),
		},
	}
}

// toStatus maps domain errors to the codes in AGENTS.md Section 9.
// Unexpected errors become INTERNAL with a fixed message so details never leak.
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrInvalidFullName),
		errors.Is(err, domain.ErrInvalidPassword), errors.Is(err, domain.ErrRoleNotAllowed),
		errors.Is(err, domain.ErrInvalidUserID), errors.Is(err, domain.ErrInvalidRole),
		errors.Is(err, domain.ErrInvalidPageSize), errors.Is(err, domain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
