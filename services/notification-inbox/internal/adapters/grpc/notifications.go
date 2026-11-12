package grpc

import (
	"context"
	"errors"
	"time"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	notificationinboxv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/notification_inbox/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/notification-inbox/internal/domain"
)

type NotificationInboxHandler struct {
	notificationinboxv1.UnimplementedNotificationInboxServiceServer
	svc *app.Notifications
}

var _ notificationinboxv1.NotificationInboxServiceServer = (*NotificationInboxHandler)(nil)

func NewNotificationInboxHandler(svc *app.Notifications) *NotificationInboxHandler {
	return &NotificationInboxHandler{svc: svc}
}

func (h *NotificationInboxHandler) ListMyNotifications(ctx context.Context, req *notificationinboxv1.ListMyNotificationsRequest) (*notificationinboxv1.ListMyNotificationsResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}

	pageSize := int(req.GetPagination().GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}
	offset := 0 // Simplified for Sprint 1

	notifications, total, err := h.svc.ListMyNotifications(ctx, caller, req.GetUnreadOnly(), pageSize, offset)
	if err != nil {
		return nil, toStatus(err)
	}

	var out []*notificationinboxv1.Notification
	for _, n := range notifications {
		out = append(out, toProto(n))
	}

	return &notificationinboxv1.ListMyNotificationsResponse{
		Notifications: out,
		Pagination:    &commonv1.PaginationResponse{TotalCount: int64(total)},
	}, nil
}

func (h *NotificationInboxHandler) MarkNotificationRead(ctx context.Context, req *notificationinboxv1.MarkNotificationReadRequest) (*notificationinboxv1.MarkNotificationReadResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}

	n, err := h.svc.MarkNotificationRead(ctx, caller, req.GetNotificationId())
	if err != nil {
		return nil, toStatus(err)
	}

	return &notificationinboxv1.MarkNotificationReadResponse{Notification: toProto(n)}, nil
}

// Internal Handler
type NotificationInboxInternalHandler struct {
	notificationinboxv1.UnimplementedNotificationInboxInternalServiceServer
	svc *app.Notifications
}

var _ notificationinboxv1.NotificationInboxInternalServiceServer = (*NotificationInboxInternalHandler)(nil)

func NewNotificationInboxInternalHandler(svc *app.Notifications) *NotificationInboxInternalHandler {
	return &NotificationInboxInternalHandler{svc: svc}
}

func (h *NotificationInboxInternalHandler) CreateNotification(ctx context.Context, req *notificationinboxv1.CreateNotificationRequest) (*notificationinboxv1.CreateNotificationResponse, error) {
	// Domain validation
	n, err := domain.NewNotification(
		req.GetRecipientId(),
		req.GetType(),
		req.GetTitle(),
		req.GetBody(),
		req.GetRelatedId(),
		req.GetSourceEventId(),
		time.Now(),
	)
	if err != nil {
		return nil, toStatus(err)
	}

	created, err := h.svc.CreateNotification(ctx, n)
	if err != nil {
		return nil, toStatus(err)
	}

	return &notificationinboxv1.CreateNotificationResponse{Notification: toProto(created)}, nil
}

// Helpers
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}

func toProto(n domain.Notification) *notificationinboxv1.Notification {
	return &notificationinboxv1.Notification{
		NotificationId: n.NotificationID,
		RecipientId:    n.RecipientID,
		Type:           n.Type,
		Title:          n.Title,
		Body:           n.Body,
		RelatedId:      n.RelatedID,
		Read:           n.Read,
		SourceEventId:  n.SourceEventID,
		CreatedAt:      timestamppb.New(n.CreatedAt),
	}
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidNotificationID),
		errors.Is(err, domain.ErrInvalidRecipientID),
		errors.Is(err, domain.ErrInvalidSourceEventID),
		errors.Is(err, domain.ErrInvalidType),
		errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrInvalidBody):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrNotOwner):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
