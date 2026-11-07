package grpcclient_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalClient_ConfiguredMetadataWinsAcrossRetries(t *testing.T) {
	for _, internal := range []bool{true, false} {
		t.Run(map[bool]string{true: "internal", false: "public"}[internal], func(t *testing.T) {
			interceptor, err := grpcclient.InternalUnaryClientInterceptor("synthetic-jt44-token", "notification-dispatcher")
			if err != nil {
				t.Fatal(err)
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-token", "untrusted", "x-caller-service", "weekly-summary"))
			ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("trace-id", "trace", "x-internal-token", "old", "x-caller-service", "old"))
			ctx = metadata.AppendToOutgoingContext(ctx, "x-internal-token", "conflict", "x-caller-service", "conflict")
			original, _ := metadata.FromOutgoingContext(ctx)
			method := "/jobtrack.auth.v1.AuthService/Login"
			if internal {
				method = "/jobtrack.auth.v1.AuthInternalService/GetUser"
			}
			calls := 0
			err = interceptor(ctx, method, nil, nil, nil, func(c context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				calls++
				md, _ := metadata.FromOutgoingContext(c)
				want := metadata.Pairs("trace-id", "trace")
				if internal {
					want.Set("x-internal-token", "synthetic-jt44-token")
					want.Set("x-caller-service", "notification-dispatcher")
				}
				if !reflect.DeepEqual(md, want) {
					t.Error("metadata did not match configured values")
				}
				if calls < 4 {
					return status.Error(codes.Unavailable, "retry")
				}
				return nil
			})
			if err != nil || calls != 4 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			after, _ := metadata.FromOutgoingContext(ctx)
			if !reflect.DeepEqual(after, original) {
				t.Error("mutated caller metadata")
			}
		})
	}
}

func TestInternalClient_Configuration(t *testing.T) {
	for _, caller := range []string{"job-alert", "notification-dispatcher", "weekly-summary", "deadline-reminder", "resume-validator", "gateway", "frontend", "", "weekly-summary "} {
		t.Run(caller, func(t *testing.T) {
			_, err := grpcclient.InternalUnaryClientInterceptor("synthetic-token", caller)
			valid := caller == "job-alert" || caller == "notification-dispatcher" || caller == "weekly-summary" || caller == "deadline-reminder" || caller == "resume-validator"
			if (err == nil) != valid {
				t.Error("unexpected caller validation result")
			}
		})
	}
	for _, token := range []string{"", "bad token", "bad\n"} {
		if _, err := grpcclient.InternalUnaryClientInterceptor(token, "weekly-summary"); err == nil {
			t.Error("accepted invalid token")
		}
	}
}

func TestInternalClient_WithoutExistingMetadata(t *testing.T) {
	interceptor, err := grpcclient.InternalUnaryClientInterceptor("synthetic-token", "weekly-summary")
	if err != nil {
		t.Fatal(err)
	}
	err = interceptor(context.Background(), "/jobtrack.auth.v1.AuthInternalService/ListUsersByRole", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		if !reflect.DeepEqual(md, metadata.Pairs("x-internal-token", "synthetic-token", "x-caller-service", "weekly-summary")) {
			t.Error("missing configured metadata")
		}
		return status.Error(codes.PermissionDenied, "downstream denial")
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatal("lost downstream status")
	}
}
