package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
)

type NotificationsRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationsRepository(pool *pgxpool.Pool) *NotificationsRepository {
	return &NotificationsRepository{pool: pool}
}

func (r *NotificationsRepository) CreateNotification(ctx context.Context, n domain.Notification) (domain.Notification, error) {
	// Try to insert. If (recipient_id, source_event_id) already exists, do nothing.
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (recipient_id, type, title, body, related_id, source_event_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (recipient_id, source_event_id) DO NOTHING`,
		n.RecipientID, n.Type, n.Title, n.Body, n.RelatedID, n.SourceEventID, n.CreatedAt)
	if err != nil {
		return domain.Notification{}, err
	}

	// Whether we inserted or it already existed, fetch the existing record to return it.
	var out domain.Notification
	err = r.pool.QueryRow(ctx, `
		SELECT notification_id, recipient_id, type, title, body, coalesce(related_id, ''), read, source_event_id, created_at
		FROM notifications
		WHERE recipient_id = $1 AND source_event_id = $2`,
		n.RecipientID, n.SourceEventID).Scan(
		&out.NotificationID, &out.RecipientID, &out.Type, &out.Title, &out.Body,
		&out.RelatedID, &out.Read, &out.SourceEventID, &out.CreatedAt)

	return out, err
}

func (r *NotificationsRepository) ListMyNotifications(ctx context.Context, recipientID string, unreadOnly bool, limit, offset int) ([]domain.Notification, int, error) {
	where := "recipient_id = $1"
	if unreadOnly {
		where += " AND read = FALSE"
	}

	var total int
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE "+where, recipientID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT notification_id, recipient_id, type, title, body, coalesce(related_id, ''), read, source_event_id, created_at
		FROM notifications
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, recipientID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifications []domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.NotificationID, &n.RecipientID, &n.Type, &n.Title, &n.Body,
			&n.RelatedID, &n.Read, &n.SourceEventID, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		notifications = append(notifications, n)
	}
	return notifications, total, rows.Err()
}

func (r *NotificationsRepository) MarkNotificationRead(ctx context.Context, notificationID, recipientID string) (domain.Notification, error) {
	// We update and return the row only if it belongs to the recipient.
	var out domain.Notification
	err := r.pool.QueryRow(ctx, `
		UPDATE notifications
		SET read = TRUE
		WHERE notification_id = $1 AND recipient_id = $2
		RETURNING notification_id, recipient_id, type, title, body, coalesce(related_id, ''), read, source_event_id, created_at`,
		notificationID, recipientID).Scan(
		&out.NotificationID, &out.RecipientID, &out.Type, &out.Title, &out.Body,
		&out.RelatedID, &out.Read, &out.SourceEventID, &out.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Did it not exist, or was it just owned by someone else?
			var ownerID string
			checkErr := r.pool.QueryRow(ctx, "SELECT recipient_id FROM notifications WHERE notification_id = $1", notificationID).Scan(&ownerID)
			if checkErr != nil {
				return domain.Notification{}, domain.ErrNotFound
			}
			if ownerID != recipientID {
				return domain.Notification{}, domain.ErrNotOwner
			}
		}
		return domain.Notification{}, err
	}
	return out, nil
}
