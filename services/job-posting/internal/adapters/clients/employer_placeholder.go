package clients

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/ports"
)

// UnavailableEmployerVerifier stands in for the EmployerService client until
// JT-57. It never reports an employer as verified, so no job can be published
// without a real verification check.
type UnavailableEmployerVerifier struct{}

var _ ports.EmployerVerifier = UnavailableEmployerVerifier{}

func (UnavailableEmployerVerifier) IsVerified(context.Context, string) (bool, error) {
	return false, domain.ErrEmployerCheckUnavailable
}
