package grpc_test

import (
	"context"
	"errors"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	messagingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	adapter "github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

const appID = "aaaaaaaa-1111-4111-8111-111111111111"
const userID = "bbbbbbbb-2222-4222-8222-222222222222"
const otherID = "cccccccc-3333-4333-8333-333333333333"

func TestMessages_StatusCodes(t *testing.T) {
	for _, method := range []string{"send", "list"} {
		for _, tc := range []struct {
			name, role, id string
			auth           bool
			repoErr        error
			code           codes.Code
		}{{"missing auth", "", "", false, nil, codes.Unauthenticated}, {"bad identity", "applicant", "bad", true, nil, codes.Unauthenticated}, {"admin", "admin", userID, true, nil, codes.PermissionDenied}, {"unknown", "applicant", userID, true, domain.ErrParticipantsNotFound, codes.NotFound}, {"infrastructure", "applicant", userID, true, errors.New("secret detail"), codes.Internal}, {"canceled", "applicant", userID, true, context.Canceled, codes.Canceled}, {"deadline", "applicant", userID, true, context.DeadlineExceeded, codes.DeadlineExceeded}} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				repo := mocks.NewMockMessageRepository(ctrl)
				p := mocks.NewMockParticipantRepository(ctrl)
				clock := mocks.NewMockClock(ctrl)
				ctx := context.Background()
				if tc.auth {
					ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-user-id", tc.id, "x-user-role", tc.role))
				}
				if tc.repoErr != nil {
					p.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{}, tc.repoErr)
				}
				h := adapter.NewMessageHandler(app.NewMessages(repo, p, clock))
				var err error
				if method == "send" {
					_, err = h.SendMessage(ctx, &messagingv1.SendMessageRequest{ApplicationId: appID, Content: "hello"})
				} else {
					_, err = h.ListMessages(ctx, &messagingv1.ListMessagesRequest{ApplicationId: appID})
				}
				require.Equal(t, tc.code, status.Code(err))
				if tc.code == codes.Internal {
					require.NotContains(t, err.Error(), "secret")
				}
			})
		}
	}
}
func TestMessages_RequestAndResponseMapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockMessageRepository(ctrl)
	p := mocks.NewMockParticipantRepository(ctrl)
	clock := mocks.NewMockClock(ctrl)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", userID, "x-user-role", "applicant"))
	h := adapter.NewMessageHandler(app.NewMessages(repo, p, clock))
	_, err := h.SendMessage(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = h.ListMessages(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	p.EXPECT().GetByApplicationID(gomock.Any(), appID).Return(domain.ApplicationParticipants{ApplicationID: appID, ApplicantID: userID, EmployerID: otherID}, nil).AnyTimes()
	clock.EXPECT().Now().Return(time.Now())
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m domain.Message) (domain.Message, error) { return m, nil })
	sent, err := h.SendMessage(ctx, &messagingv1.SendMessageRequest{ApplicationId: appID, Content: " hello "})
	require.NoError(t, err)
	require.Equal(t, "hello", sent.Message.Content)
	require.Equal(t, userID, sent.Message.SenderId)
	require.Equal(t, otherID, sent.Message.RecipientId)
	require.NoError(t, sent.Message.Timestamps.CreatedAt.CheckValid())
	_, err = h.SendMessage(ctx, &messagingv1.SendMessageRequest{ApplicationId: appID})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = h.ListMessages(ctx, &messagingv1.ListMessagesRequest{ApplicationId: appID, Pagination: &commonv1.PaginationRequest{PageToken: "-1"}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	for _, tc := range []struct {
		offset string
		total  int64
		count  int
		next   string
	}{{"0", 3, 2, "2"}, {"2", 3, 1, ""}, {"9", 3, 0, ""}, {"0", 0, 0, ""}} {
		items := make([]domain.Message, tc.count)
		repo.EXPECT().List(gomock.Any(), appID, gomock.Any()).Return(items, tc.total, nil)
		out, err := h.ListMessages(ctx, &messagingv1.ListMessagesRequest{ApplicationId: appID, Pagination: &commonv1.PaginationRequest{PageSize: 2, PageToken: tc.offset}})
		require.NoError(t, err)
		require.Len(t, out.Messages, tc.count)
		require.Equal(t, tc.next, out.Pagination.NextPageToken)
		require.Equal(t, tc.total, out.Pagination.TotalCount)
	}
}
