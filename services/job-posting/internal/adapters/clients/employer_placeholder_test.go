package clients_test

import (
	"context"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/adapters/clients"
	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnavailableEmployerVerifier_NeverVerifies(t *testing.T) {
	verified, err := clients.UnavailableEmployerVerifier{}.IsVerified(context.Background(), "3f2b8c1e-1111-4222-8333-444455556666")
	require.ErrorIs(t, err, domain.ErrEmployerCheckUnavailable)
	assert.False(t, verified)
}
