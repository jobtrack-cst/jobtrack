package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports"
)

type Messages struct{ pool *pgxpool.Pool }

var _ ports.MessageRepository = (*Messages)(nil)

func NewMessages(p *pgxpool.Pool) *Messages { return &Messages{p} }

const columns = "message_id,application_id,sender_id,recipient_id,content,created_at,updated_at"

func scanMessage(row pgx.Row) (domain.Message, error) {
	var m domain.Message
	err := row.Scan(&m.ID, &m.ApplicationID, &m.SenderID, &m.RecipientID, &m.Content, &m.CreatedAt, &m.UpdatedAt)
	m.CreatedAt = m.CreatedAt.UTC()
	m.UpdatedAt = m.UpdatedAt.UTC()
	return m, err
}
func dbError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("messaging database operation failed")
}
func (r *Messages) Create(ctx context.Context, m domain.Message) (domain.Message, error) {
	m, err := scanMessage(r.pool.QueryRow(ctx, `INSERT INTO messages (`+columns+`) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+columns, m.ID, m.ApplicationID, m.SenderID, m.RecipientID, m.Content, m.CreatedAt, m.UpdatedAt))
	if err != nil {
		return domain.Message{}, dbError(ctx)
	}
	return m, nil
}
func (r *Messages) List(ctx context.Context, id string, p domain.Page) ([]domain.Message, int64, error) {
	// Count and page share one snapshot even during concurrent sends.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, dbError(ctx)
	}
	defer tx.Rollback(ctx)
	var total int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM messages WHERE application_id=$1`, id).Scan(&total); err != nil {
		return nil, 0, dbError(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT `+columns+` FROM messages WHERE application_id=$1 ORDER BY created_at,message_id LIMIT $2 OFFSET $3`, id, p.Size, p.Offset)
	if err != nil {
		return nil, 0, dbError(ctx)
	}
	defer rows.Close()
	result := []domain.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, dbError(ctx)
		}
		result = append(result, m)
	}
	if rows.Err() != nil {
		return nil, 0, dbError(ctx)
	}
	if tx.Commit(ctx) != nil {
		return nil, 0, dbError(ctx)
	}
	return result, total, nil
}
