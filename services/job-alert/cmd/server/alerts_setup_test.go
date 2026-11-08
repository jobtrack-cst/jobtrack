package main

import (
	"context"
	"io"
	"testing"
	"time"

	jobalertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestServe_RegistersJobAlertPublicRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockAlertRepository(ctrl)
	owner := "11111111-1111-1111-1111-111111111111"
	repo.EXPECT().List(gomock.Any(), owner, domain.Page{Size: 20}).Return([]domain.Alert{}, int64(0), nil)
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("job-alert", io.Discard), app.NewHealth(), grpcListener, httpListener, app.NewAlerts(repo, nil, nil))
	}()
	defer func() { cancel(); require.NoError(t, awaitServer(t, done)) }()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	require.NoError(t, err)
	defer conn.Close()
	rpcCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	rpcCtx = metadata.NewIncomingContext(rpcCtx, metadata.Pairs("x-user-id", owner, "x-user-role", "applicant"))
	response, err := jobalertv1.NewJobAlertServiceClient(conn).ListMyJobAlerts(rpcCtx, &jobalertv1.ListMyJobAlertsRequest{})
	require.NoError(t, err)
	require.Empty(t, response.JobAlerts)
	require.NotNil(t, response.Pagination)
}
