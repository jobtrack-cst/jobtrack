package grpc

import (
	"context"
	"errors"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	messagingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strconv"
)

type MessageHandler struct {
	messagingv1.UnimplementedMessagingServiceServer
	messages *app.Messages
}

func NewMessageHandler(m *app.Messages) *MessageHandler { return &MessageHandler{messages: m} }
func (h *MessageHandler) SendMessage(ctx context.Context, r *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	m, err := h.messages.Send(ctx, c, r.GetApplicationId(), r.GetContent())
	if err != nil {
		return nil, toStatus(err)
	}
	return &messagingv1.SendMessageResponse{Message: messageProto(m)}, nil
}
func (h *MessageHandler) ListMessages(ctx context.Context, r *messagingv1.ListMessagesRequest) (*messagingv1.ListMessagesResponse, error) {
	c, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	items, total, page, err := h.messages.List(ctx, c, r.GetApplicationId(), r.GetPagination().GetPageSize(), r.GetPagination().GetPageToken())
	if err != nil {
		return nil, toStatus(err)
	}
	out := &messagingv1.ListMessagesResponse{Pagination: &commonv1.PaginationResponse{TotalCount: total}}
	for _, m := range items {
		out.Messages = append(out.Messages, messageProto(m))
	}
	if int64(len(items)) < total-page.Offset {
		out.Pagination.NextPageToken = strconv.FormatInt(page.Offset+int64(len(items)), 10)
	}
	return out, nil
}
func messageProto(m domain.Message) *messagingv1.Message {
	return &messagingv1.Message{MessageId: m.ID, ApplicationId: m.ApplicationID, SenderId: m.SenderID, RecipientId: m.RecipientID, Content: m.Content, Timestamps: &commonv1.ResourceTimestamps{CreatedAt: timestamppb.New(m.CreatedAt), UpdatedAt: timestamppb.New(m.UpdatedAt)}}
}
func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalidPagination), errors.Is(err, domain.ErrInvalidApplicationID), errors.Is(err, domain.ErrInvalidContent):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrParticipantsNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
