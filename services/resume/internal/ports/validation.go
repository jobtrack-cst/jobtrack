package ports

//go:generate go tool mockgen -destination=mocks/validation_mock.go -package=mocks . ValidationRepository

import (
	"context"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/resume/internal/domain"
)

type ValidationRepository interface {
	// ApplyValidation commits event-ID deduplication and a PENDING->final update
	// atomically. Distinct events targeting a final resume are recorded but ignored.
	// Unknown resumes and applicant mismatches return errors and record nothing;
	// a future transport must decide their retry/poison disposition.
	ApplyValidation(context.Context, domain.ValidationResult, time.Time) (domain.ValidationOutcome, error)
}
