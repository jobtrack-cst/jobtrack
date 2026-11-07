package grpc_test

import (
	"context"
	"errors"
	"testing"

	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	jobalertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	adapter "github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/grpc"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const owner = "11111111-1111-1111-1111-111111111111"

func identity(role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", owner, "x-user-role", role))
}
func TestAlerts_AllRPCsCheckTrustedIdentityAndRole(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
		code codes.Code
	}{
		{"missing", context.Background(), codes.Unauthenticated}, {"unknown", identity("other"), codes.Unauthenticated}, {"employer", identity("employer"), codes.PermissionDenied}, {"admin", identity("admin"), codes.PermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := adapter.NewAlertHandler(app.NewAlerts(mocks.NewMockAlertRepository(ctrl), nil, nil))
			_, err := h.CreateJobAlert(tc.ctx, &jobalertv1.CreateJobAlertRequest{Location: "Thimphu"})
			require.Equal(t, tc.code, status.Code(err))
			_, err = h.ListMyJobAlerts(tc.ctx, &jobalertv1.ListMyJobAlertsRequest{})
			require.Equal(t, tc.code, status.Code(err))
			_, err = h.DeleteJobAlert(tc.ctx, &jobalertv1.DeleteJobAlertRequest{AlertId: owner})
			require.Equal(t, tc.code, status.Code(err))
		})
	}
}
func TestAlerts_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{domain.ErrExists, codes.AlreadyExists}, {domain.ErrLimit, codes.FailedPrecondition}, {domain.ErrPermissionDenied, codes.PermissionDenied}, {domain.ErrNotFound, codes.NotFound}, {domain.ErrInvalid, codes.InvalidArgument}, {context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded}, {errors.New("secret database details"), codes.Internal},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockAlertRepository(ctrl)
			repo.EXPECT().Delete(gomock.Any(), owner, owner).Return(tc.err)
			h := adapter.NewAlertHandler(app.NewAlerts(repo, nil, nil))
			_, err := h.DeleteJobAlert(identity("applicant"), &jobalertv1.DeleteJobAlertRequest{AlertId: owner})
			require.Equal(t, tc.code, status.Code(err))
			if tc.code == codes.Internal {
				require.NotContains(t, err.Error(), "secret")
			}
		})
	}
}
func TestAlerts_ProtoResponsesAndPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAlertRepository(ctrl)
	h := adapter.NewAlertHandler(app.NewAlerts(repo, nil, nil))
	ctx := identity("applicant")
	repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, a domain.Alert) (domain.Alert, error) { return a, nil })
	created, err := h.CreateJobAlert(ctx, &jobalertv1.CreateJobAlertRequest{Location: "Thimphu"})
	require.NoError(t, err)
	require.Equal(t, owner, created.JobAlert.ApplicantId)
	require.Equal(t, "Thimphu", created.JobAlert.Location)
	require.NoError(t, created.JobAlert.Timestamps.CreatedAt.CheckValid())
	for _, tc := range []struct {
		offset string
		items  int
		next   string
	}{{"", 1, "1"}, {"1", 1, ""}, {"9223372036854775807", 0, ""}} {
		page, _ := domain.ParsePage(1, tc.offset)
		items := make([]domain.Alert, tc.items)
		repo.EXPECT().List(ctx, owner, page).Return(items, int64(2), nil)
		out, err := h.ListMyJobAlerts(ctx, &jobalertv1.ListMyJobAlertsRequest{Pagination: &commonv1.PaginationRequest{PageSize: 1, PageToken: tc.offset}})
		require.NoError(t, err)
		require.Equal(t, tc.next, out.Pagination.NextPageToken)
		require.EqualValues(t, 2, out.Pagination.TotalCount)
	}
	repo.EXPECT().Delete(ctx, created.JobAlert.AlertId, owner).Return(nil)
	_, err = h.DeleteJobAlert(ctx, &jobalertv1.DeleteJobAlertRequest{AlertId: created.JobAlert.AlertId})
	require.NoError(t, err)
	_, err = h.CreateJobAlert(ctx, &jobalertv1.CreateJobAlertRequest{CategoryIds: []string{owner}})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = h.ListMyJobAlerts(ctx, &jobalertv1.ListMyJobAlertsRequest{Pagination: &commonv1.PaginationRequest{PageSize: -1}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = h.CreateJobAlert(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = h.ListMyJobAlerts(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = h.DeleteJobAlert(ctx, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
