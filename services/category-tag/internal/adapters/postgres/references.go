package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/ports"
)

// References reads the seeded category, tag and location tables. It never
// writes: the lists are read-only and change only through migrations. The
// adapter never owns the pool.
type References struct{ pool *pgxpool.Pool }

var _ ports.ReferenceRepository = (*References)(nil)

func NewReferences(pool *pgxpool.Pool) *References { return &References{pool: pool} }

// Sorted by name ignoring case, so "gRPC" sits next to "Git"; COLLATE "C"
// gives the same order whatever the database locale. Names are unique
// ignoring case, and the ID only makes the order total.
const orderBy = ` ORDER BY lower(name) COLLATE "C", id`

// source maps a kind to a fixed subquery, so no request value ever becomes
// part of the SQL text.
func source(kind domain.Kind) (string, error) {
	switch kind {
	case domain.KindCategory:
		return `(SELECT category_id AS id, name FROM categories) AS items`, nil
	case domain.KindTag:
		return `(SELECT tag_id AS id, name FROM tags) AS items`, nil
	case domain.KindLocation:
		return `(SELECT location_id AS id, name FROM locations) AS items`, nil
	default:
		return "", errors.New("unknown reference kind")
	}
}

// List runs the count and the page in one read-only snapshot, so
// total_count matches the rows returned (as in job-posting).
func (r *References) List(ctx context.Context, kind domain.Kind, page domain.Page) (items []domain.Item, total int, err error) {
	from, err := source(kind)
	if err != nil {
		return nil, 0, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list reference items failed")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+from).Scan(&total); err != nil {
		return nil, 0, hideDriverError(ctx, "count reference items failed")
	}
	rows, err := tx.Query(ctx, `SELECT id::text, name FROM `+from+orderBy+` LIMIT $1 OFFSET $2`, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, hideDriverError(ctx, "list reference items failed")
	}
	items, err = scanItems(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *References) Resolve(ctx context.Context, kind domain.Kind, ids []string) ([]domain.Item, error) {
	from, err := source(kind)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []domain.Item{}, nil
	}
	// The app layer passes only validated UUIDs, so the cast cannot fail.
	rows, err := r.pool.Query(ctx, `SELECT id::text, name FROM `+from+` WHERE id = ANY($1::uuid[])`+orderBy, ids)
	if err != nil {
		return nil, hideDriverError(ctx, "resolve reference names failed")
	}
	return scanItems(ctx, rows)
}

func scanItems(ctx context.Context, rows pgx.Rows) ([]domain.Item, error) {
	defer rows.Close()
	items := []domain.Item{}
	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, hideDriverError(ctx, "read reference items failed")
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, hideDriverError(ctx, "read reference items failed")
	}
	return items, nil
}

// hideDriverError keeps context cancellation visible but hides driver details.
func hideDriverError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
