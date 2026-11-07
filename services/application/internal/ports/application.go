package ports

import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=application.go -destination=mocks/application.go -package=mocks

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
