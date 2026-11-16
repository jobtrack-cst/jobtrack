package clients_test

import (
	"context"
	categoryv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/adapters/clients"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
)

type categoryServer struct {
	categoryv1.UnimplementedCategoryTagInternalServiceServer
}

func (categoryServer) ResolveNames(_ context.Context, r *categoryv1.ResolveNamesRequest) (*categoryv1.ResolveNamesResponse, error) {
	return &categoryv1.ResolveNamesResponse{Categories: []*categoryv1.Category{{CategoryId: r.CategoryIds[0], Name: "Engineering"}}}, nil
}
func TestNames_AuthenticatedInternalRPCAndMissingIDs(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	defer listener.Close()
	auth, err := authctx.InternalUnaryServerInterceptor("synthetic-token")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(auth))
	categoryv1.RegisterCategoryTagInternalServiceServer(server, categoryServer{})
	go server.Serve(listener)
	defer server.Stop()
	interceptor, err := grpcclient.InternalUnaryClientInterceptor("synthetic-token", "job-alert")
	require.NoError(t, err)
	conn, err := grpc.NewClient("passthrough:///category-tag", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(interceptor))
	require.NoError(t, err)
	defer conn.Close()
	resolver := clients.NewNames(categoryv1.NewCategoryTagInternalServiceClient(conn))
	id := "10000000-0000-4000-8000-000000000001"
	names, err := resolver.ResolveNames(context.Background(), []string{id}, nil)
	require.NoError(t, err)
	require.Equal(t, "Engineering", names.Categories[id])
	require.NoError(t, names.Validate([]string{id}, nil))
	require.Error(t, names.Validate(nil, []string{id}))
}
