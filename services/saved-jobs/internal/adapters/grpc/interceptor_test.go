package grpc

import (
	"context"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testToken = "test-token-visible-ascii"

func TestInternalInterceptor_MissingToken(t *testing.T) {
	ic, err := authctx.InternalUnaryServerInterceptor(testToken)
	if err != nil {
		t.Fatal(err)
	}
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers",
	}
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	_, err = ic(context.Background(), nil, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestInternalInterceptor_WrongToken(t *testing.T) {
	ic, _ := authctx.InternalUnaryServerInterceptor(testToken)
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers",
	}
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-internal-token", "wrong-token-visible-ascii",
		"x-caller-service", "deadline-reminder",
	))
	_, err := ic(ctx, nil, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected UNAUTHENTICATED, got %v", err)
	}
}

func TestInternalInterceptor_WrongCaller(t *testing.T) {
	ic, _ := authctx.InternalUnaryServerInterceptor(testToken)
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers",
	}
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-internal-token", testToken,
		"x-caller-service", "some-other-service",
	))
	_, err := ic(ctx, nil, info, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestInternalInterceptor_MissingCaller(t *testing.T) {
	ic, _ := authctx.InternalUnaryServerInterceptor(testToken)
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers",
	}
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-internal-token", testToken,
	))
	_, err := ic(ctx, nil, info, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestInternalInterceptor_ValidDeadlineReminder(t *testing.T) {
	ic, _ := authctx.InternalUnaryServerInterceptor(testToken)
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers",
	}
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-internal-token", testToken,
		"x-caller-service", "deadline-reminder",
	))
	resp, err := ic(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if resp != "ok" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestInternalInterceptor_PublicMethodBypassesToken(t *testing.T) {
	ic, _ := authctx.InternalUnaryServerInterceptor(testToken)
	info := &grpc.UnaryServerInfo{
		FullMethod: "/jobtrack.saved_jobs.v1.SavedJobsService/SaveJob",
	}
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "public-ok", nil
	}
	_, err := ic(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("public method should not need token, got %v", err)
	}
	if !called {
		t.Error("public handler was not called")
	}
}
