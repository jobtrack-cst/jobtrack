package app

import (
	"context"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/ports"
)

type Notifications struct {
	repo ports.Repository
}

func NewNotifications(repo ports.Repository) *Notifications {
	return &Notifications{repo: repo}
}

// CreateNotification is for the internal API. It doesn't check roles because
// the internal gRPC interceptor handles the INTERNAL_TOKEN and caller checks.
func (s *Notifications) CreateNotification(ctx context.Context, n domain.Notification) (domain.Notification, error) {
	return s.repo.CreateNotification(ctx, n)
}

func (s *Notifications) ListMyNotifications(ctx context.Context, caller domain.Caller, unreadOnly bool, pageSize, pageOffset int) ([]domain.Notification, int, error) {
	return s.repo.ListMyNotifications(ctx, caller.UserID, unreadOnly, pageSize, pageOffset)
}

func (s *Notifications) MarkNotificationRead(ctx context.Context, caller domain.Caller, notificationID string) (domain.Notification, error) {
	if !domain.IsUUID(notificationID) {
		return domain.Notification{}, domain.ErrInvalidNotificationID
	}
	return s.repo.MarkNotificationRead(ctx, notificationID, caller.UserID)
}
