package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	categorytagv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

const stubTagID = "20000000-0000-4000-8000-000000000001"

// stubReferences is a fixed repository for wiring tests; the real one is
// covered by the Testcontainers integration test.
type stubReferences struct{}

func (stubReferences) List(context.Context, domain.Kind, domain.Page) ([]domain.Item, int, error) {
	return []domain.Item{{ID: stubTagID, Name: "Go"}}, 1, nil
}

func (stubReferences) Resolve(_ context.Context, _ domain.Kind, ids []string) ([]domain.Item, error) {
	return []domain.Item{{ID: ids[0], Name: "Go"}}, nil
}

// recordingRegistrar keeps what registerCategoryTag registers.
type recordingRegistrar struct {
	descs []*grpc.ServiceDesc
	impls []any
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.descs = append(r.descs, desc)
	r.impls = append(r.impls, impl)
}

func TestRegisterCategoryTag_RegistersPublicAndInternalServices(t *testing.T) {
	var reg recordingRegistrar
	registerCategoryTag(stubReferences{})(&reg)
	if len(reg.descs) != 2 ||
		reg.descs[0].ServiceName != "jobtrack.category_tag.v1.CategoryTagService" ||
		reg.descs[1].ServiceName != "jobtrack.category_tag.v1.CategoryTagInternalService" {
		t.Fatalf("registered %v, want CategoryTagService and CategoryTagInternalService", reg.descs)
	}
	if _, ok := reg.impls[0].(categorytagv1.CategoryTagServiceServer); !ok {
		t.Fatalf("registered %T, not a CategoryTagServiceServer", reg.impls[0])
	}
	if _, ok := reg.impls[1].(categorytagv1.CategoryTagInternalServiceServer); !ok {
		t.Fatalf("registered %T, not a CategoryTagInternalServiceServer", reg.impls[1])
	}
}

func TestInternalServerOptions_RejectsInvalidTokens(t *testing.T) {
	for _, token := range []string{"", "private-secret value", "private-secreté"} {
		opts, err := internalServerOptions(token)
		if err == nil || opts != nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("token %q: opts=%v err=%v", token, opts, err)
		}
	}
	if opts, err := internalServerOptions("valid-token"); err != nil || len(opts) != 1 {
		t.Fatalf("valid token: opts=%v err=%v", opts, err)
	}
}

// End to end over a real gRPC server with the real JT-44 interceptor, as run
// wires it: only job-alert with the right token reaches ResolveNames, and the
// public list RPCs need no internal token.
func TestServe_InternalInterceptorGuardsOnlyTheInternalService(t *testing.T) {
	const token = "test-internal-token"
	serverOpts, err := internalServerOptions(token)
	if err != nil {
		t.Fatal(err)
	}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("test", io.Discard), app.NewHealth(), registerCategoryTag(stubReferences{}),
			grpcListener, httpListener, serverOpts...)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})

	dial := func(t *testing.T, interceptor grpc.UnaryClientInterceptor) *grpc.ClientConn {
		t.Helper()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if interceptor != nil {
			opts = append(opts, grpc.WithChainUnaryInterceptor(interceptor))
		}
		conn, err := grpc.NewClient(grpcListener.Addr().String(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	internalClient := func(t *testing.T, token, caller string) *grpc.ClientConn {
		t.Helper()
		interceptor, err := grpcclient.InternalUnaryClientInterceptor(token, caller)
		if err != nil {
			t.Fatal(err)
		}
		return dial(t, interceptor)
	}
	rpcCtx := func(t *testing.T) context.Context {
		c, stop := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(stop)
		return c
	}
	resolveReq := &categorytagv1.ResolveNamesRequest{TagIds: []string{stubTagID}}

	for _, tt := range []struct {
		name string
		conn func(t *testing.T) *grpc.ClientConn
		want codes.Code
	}{
		{"job-alert with the right token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, token, "job-alert") }, codes.OK},
		{"wrong token", func(t *testing.T) *grpc.ClientConn { return internalClient(t, "wrong-token", "job-alert") }, codes.Unauthenticated},
		{"another internal caller", func(t *testing.T) *grpc.ClientConn { return internalClient(t, token, "deadline-reminder") }, codes.PermissionDenied},
		{"public client without a token", func(t *testing.T) *grpc.ClientConn { return dial(t, grpcclient.UnaryClientInterceptor()) }, codes.Unauthenticated},
	} {
		t.Run("ResolveNames: "+tt.name, func(t *testing.T) {
			resp, err := categorytagv1.NewCategoryTagInternalServiceClient(tt.conn(t)).ResolveNames(rpcCtx(t), resolveReq)
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && (len(resp.GetTags()) != 1 || resp.GetTags()[0].GetName() != "Go") {
				t.Fatalf("response %v, want the Go tag", resp)
			}
		})
	}

	// The public RPCs need no internal token. These headers stand in for the
	// identity the gateway forwards.
	for _, tt := range []struct {
		name string
		md   []string
		want codes.Code
	}{
		{"applicant", []string{"x-user-id", "22222222-2222-4222-8222-222222222222", "x-user-role", "applicant"}, codes.OK},
		{"no identity", nil, codes.Unauthenticated},
	} {
		t.Run("ListTags as "+tt.name, func(t *testing.T) {
			c := rpcCtx(t)
			if tt.md != nil {
				c = metadata.AppendToOutgoingContext(c, tt.md...)
			}
			resp, err := categorytagv1.NewCategoryTagServiceClient(dial(t, nil)).ListTags(c, &categorytagv1.ListTagsRequest{})
			if status.Code(err) != tt.want {
				t.Fatalf("code=%v (%v), want %v", status.Code(err), err, tt.want)
			}
			if tt.want == codes.OK && resp.GetPagination().GetTotalCount() != 1 {
				t.Fatalf("response %v, want total_count 1", resp)
			}
		})
	}
}
