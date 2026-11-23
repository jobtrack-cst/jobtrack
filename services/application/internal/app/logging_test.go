package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/application/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/application/internal/ports/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestApplication_StorageFailureRetainsCauseUntilAdapter(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	failure := errors.New("password=private")
	repo := mocks.NewMockRepository(gomock.NewController(t))
	repo.EXPECT().Get(gomock.Any(), id).Return(domain.Application{}, failure)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", "applicant"))
	_, err := app.NewService(repo, nil, nil, nil).Get(ctx, id)
	require.ErrorIs(t, err, failure)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "application storage failed", err.Error())
}
