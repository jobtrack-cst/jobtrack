package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

func TestHealthStatus_IsHealthy(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status domain.HealthStatus
		want   bool
	}{
		{"unknown zero value", domain.Unknown, false},
		{"serving", domain.Serving, true},
		{"not serving", domain.NotServing, false},
		{"unrecognized status", domain.HealthStatus(255), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsHealthy())
		})
	}
}
