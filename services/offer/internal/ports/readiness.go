package ports

import "context"

// Readiness reports whether a required dependency can accept requests.
type Readiness interface {
	Check(context.Context) error
}
