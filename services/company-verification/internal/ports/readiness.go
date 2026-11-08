package ports

import "context"

//go:generate go tool mockgen -destination=mocks/readiness_mock.go -package=mocks . Readiness

// Readiness reports whether a required dependency can accept requests.
type Readiness interface {
	Check(context.Context) error
}
