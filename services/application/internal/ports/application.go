package ports

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/application.go -package=mocks . Repository,JobPosting,Resume

// Repository atomically stores each successful write and its event in the outbox.
type Repository interface {
	Create(context.Context, domain.Application) (domain.Application, error)
	Get(context.Context, string) (domain.Application, error)
	Withdraw(context.Context, string, string) (domain.Application, error)
	List(context.Context, Filter) ([]domain.Application, int64, error)
}

type Filter struct {
	ApplicantID, EmployerID, JobID string
	Limit, Offset                  int
}

type JobPosting interface {
	GetJob(context.Context, string) (domain.Job, error)
}

type Resume interface {
	GetResume(context.Context, string) (domain.Resume, error)
}
