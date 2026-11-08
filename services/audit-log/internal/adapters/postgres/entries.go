package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/audit-log/internal/ports"
)

const entryColumns = "event_id, event_type, producer, occurred_at, data_json"

// Entries stores audit entries and processed event IDs in the audit_log
// database. The adapter never owns the pool.
type Entries struct{ pool *pgxpool.Pool }

var _ ports.EntryRepository = (*Entries)(nil)

func NewEntries(pool *pgxpool.Pool) *Entries { return &Entries{pool: pool} }

// RecordEntry claims the event_id first. When two deliveries of the same
// event race, the second insert waits for the first transaction and then
// does nothing, so exactly one entry is stored. If the entry insert fails,
// the claim rolls back too and a redelivery can try again.
func (r *Entries) RecordEntry(ctx context.Context, e domain.Entry) (bool, error) {
	recorded := false
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO processed_events (event_id) VALUES ($1)
			 ON CONFLICT (event_id) DO NOTHING`, e.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO audit_entries (`+entryColumns+`) VALUES ($1, $2, $3, $4, $5)`,
			e.EventID, e.EventType, e.Producer, e.OccurredAt, e.DataJSON); err != nil {
			return err
		}
		recorded = true
		return nil
	})
	if err != nil {
		// Driver errors can contain SQL literals, so return a fixed message.
		return false, safe(ctx, "record audit entry failed")
	}
	return recorded, nil
}

// ListEntries runs the count and the page in one read-only snapshot, so
// total_count matches the rows returned.
func (r *Entries) ListEntries(ctx context.Context, f domain.ListFilter, page domain.Page) ([]domain.Entry, int64, error) {
	where, args := entryWhere(f)
	var items []domain.Entry
	var total int64
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_entries`+where, args...).Scan(&total); err != nil {
			return err
		}
		n := len(args)
		rows, err := tx.Query(ctx,
			`SELECT `+entryColumns+` FROM audit_entries`+where+
				` ORDER BY occurred_at DESC, event_id DESC`+
				` LIMIT $`+strconv.Itoa(n+1)+` OFFSET $`+strconv.Itoa(n+2),
			append(args, page.Limit, page.Offset)...)
		if err != nil {
			return err
		}
		items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Entry, error) {
			return scanEntry(row)
		})
		return err
	})
	if err != nil {
		return nil, 0, safe(ctx, "list audit entries failed")
	}
	return items, total, nil
}

// entryWhere builds the WHERE clause from fixed strings only; every filter
// value is a bind parameter. start is inclusive and end exclusive, as the
// contract says.
func entryWhere(f domain.ListFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, cond+" $"+strconv.Itoa(len(args)))
	}
	if f.EventType != "" {
		add("event_type =", f.EventType)
	}
	if f.Start != nil {
		add("occurred_at >=", *f.Start)
	}
	if f.End != nil {
		add("occurred_at <", *f.End)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func scanEntry(row pgx.Row) (domain.Entry, error) {
	var e domain.Entry
	if err := row.Scan(&e.EventID, &e.EventType, &e.Producer, &e.OccurredAt, &e.DataJSON); err != nil {
		return domain.Entry{}, err
	}
	// Return UTC, as AGENTS.md Section 9 requires.
	e.OccurredAt = e.OccurredAt.UTC()
	return e, nil
}

// safe keeps context cancellation visible but hides driver details.
func safe(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}
