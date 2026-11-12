package ports

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
)

//go:generate go tool mockgen -destination=mocks/notifications_mock.go -package=mocks . Repository

type Repository interface {
	// CreateNotification inserts a new notification. If a notification with the
	// same recipient_id and source_event_id already exists, it returns the
	// existing one without throwing an error (idempotency).
	CreateNotification(ctx context.Context, n domain.Notification) (domain.Notification, error)

	// ListMyNotifications returns the recipient's notifications, newest first.
	// If unreadOnly is true, it only returns unread notifications.
	ListMyNotifications(ctx context.Context, recipientID string, unreadOnly bool, limit, offset int) ([]domain.Notification, int, error)

	// MarkNotificationRead marks a notification as read.
	// It returns domain.ErrNotFound if the ID doesn't exist, or domain.ErrNotOwner
	// if the notification belongs to someone else.
	MarkNotificationRead(ctx context.Context, notificationID, recipientID string) (domain.Notification, error)
}
