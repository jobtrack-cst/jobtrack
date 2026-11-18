package domain_test

import (
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-posting/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRemovalPublishesClose_OnlyForPublishedJobs(t *testing.T) {
	tests := []struct {
		name     string
		previous domain.Status
		want     bool
	}{
		{"draft", domain.StatusDraft, false},
		{"published", domain.StatusPublished, true},
		{"closed", domain.StatusClosed, false},
		{"removed", domain.StatusRemoved, false},
		{"unspecified", domain.StatusUnspecified, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.RemovalPublishesClose(tt.previous))
		})
	}
}
