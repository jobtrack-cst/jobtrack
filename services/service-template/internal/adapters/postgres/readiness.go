package postgres

import (
	"context"
	"errors"

	"github.com/jobtrack-cst/jobtrack/services/service-template/internal/ports"
)

// Pinger is implemented by the existing pgx pool; the adapter never owns it.
type Pinger interface{ Ping(context.Context) error }

type Readiness struct{ pool Pinger }

var _ ports.Readiness = (*Readiness)(nil)

func NewReadiness(pool Pinger) *Readiness { return &Readiness{pool: pool} }

func (r *Readiness) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.pool == nil {
		return errors.New("database readiness unavailable")
	}
	if err := r.pool.Ping(ctx); err != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("database readiness check failed")
	}
	return nil
}
