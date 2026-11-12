package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/references_mock.go -package=mocks . ReferenceRepository

// ReferenceRepository reads the seeded reference lists. It has no write
// methods: the lists are read-only and change only through migrations.
type ReferenceRepository interface {
	// List returns one page of the kind's items sorted by name ignoring case,
	// then by ID, and the total number of items of that kind.
	List(ctx context.Context, kind domain.Kind, page domain.Page) (items []domain.Item, total int, err error)
	// Resolve returns the items of the kind whose IDs are in ids, sorted like
	// List. IDs with no item are left out. ids are lowercase UUIDs.
	Resolve(ctx context.Context, kind domain.Kind, ids []string) ([]domain.Item, error)
}
