package clients

import (
	"context"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Users reads applicant names from auth (JT-114). Its connection must install
// grpcclient.InternalUnaryClientInterceptor(INTERNAL_TOKEN, "application"),
// because GetUser is an internal RPC.
type Users struct {
	client authv1.AuthInternalServiceClient
}

var _ ports.Users = (*Users)(nil)

func NewUsers(client authv1.AuthInternalServiceClient) *Users { return &Users{client: client} }

func (c *Users) GetUserName(ctx context.Context, userID string) (string, error) {
	r, err := c.client.GetUser(ctx, &authv1.GetUserRequest{UserId: userID})
	if err != nil {
		return "", err
	}
	u := r.GetUser()
	if u == nil || u.GetUserId() != userID || !domain.ValidApplicantName(u.GetFullName()) {
		return "", status.Error(codes.Internal, "invalid user dependency response")
	}
	return u.GetFullName(), nil
}
