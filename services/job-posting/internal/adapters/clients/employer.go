package clients

import (
	"context"
	"fmt"

	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EmployerClient checks verification through EmployerService.GetCompanyProfile.
// Its connection must use grpcclient.UnaryClientInterceptor, which forwards the
// caller's x-user-id and x-user-role (AGENTS.md Section 6).
type EmployerClient struct {
	client employerv1.EmployerServiceClient
}

var _ ports.EmployerVerifier = (*EmployerClient)(nil)

func NewEmployerClient(client employerv1.EmployerServiceClient) *EmployerClient {
	return &EmployerClient{client: client}
}

// IsVerified fails closed: only an explicit VERIFIED status returns true.
// NOT_FOUND means no profile, which the port defines as not verified. Any
// other failure wraps domain.ErrEmployerCheckFailed so it is never read as a
// decision. Only the status code is kept, never the downstream message.
func (c *EmployerClient) IsVerified(ctx context.Context, employerID string) (bool, error) {
	resp, err := c.client.GetCompanyProfile(ctx, &employerv1.GetCompanyProfileRequest{EmployerId: employerID})
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: GetCompanyProfile returned %s", domain.ErrEmployerCheckFailed, status.Code(err))
	}
	switch s := resp.GetProfile().GetVerificationStatus(); s {
	case employerv1.VerificationStatus_VERIFICATION_STATUS_VERIFIED:
		return true, nil
	case employerv1.VerificationStatus_VERIFICATION_STATUS_UNVERIFIED,
		employerv1.VerificationStatus_VERIFICATION_STATUS_REJECTED:
		return false, nil
	default:
		return false, fmt.Errorf("%w: unexpected verification status %d", domain.ErrEmployerCheckFailed, s)
	}
}
