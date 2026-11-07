package authctx_test

import (
	"context"
	"testing"

	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const token = "synthetic-jt44-token"
const getUser = "/jobtrack.auth.v1.AuthInternalService/GetUser"

func TestInternalInterceptor_SecurityBoundaries(t *testing.T) {
	interceptor, err := authctx.InternalUnaryServerInterceptor(token)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, method    string
		tokens, callers []string
		code            codes.Code
	}{
		{"missing token", getUser, nil, []string{"notification-dispatcher"}, codes.Unauthenticated},
		{"empty token", getUser, []string{""}, nil, codes.Unauthenticated},
		{"wrong token", getUser, []string{"wrong"}, nil, codes.Unauthenticated},
		{"same length wrong token", getUser, []string{"synthetic-jt44-tokex"}, nil, codes.Unauthenticated},
		{"whitespace token", getUser, []string{token + " "}, nil, codes.Unauthenticated},
		{"control token", getUser, []string{token + "\n"}, nil, codes.Unauthenticated},
		{"duplicate token", getUser, []string{token, token}, []string{"notification-dispatcher"}, codes.Unauthenticated},
		{"conflicting token", getUser, []string{token, "wrong"}, nil, codes.Unauthenticated},
		{"missing caller", getUser, []string{token}, nil, codes.PermissionDenied},
		{"empty caller", getUser, []string{token}, []string{""}, codes.PermissionDenied},
		{"wrong rpc caller", getUser, []string{token}, []string{"weekly-summary"}, codes.PermissionDenied},
		{"duplicate caller", getUser, []string{token}, []string{"notification-dispatcher", "notification-dispatcher"}, codes.PermissionDenied},
		{"conflicting caller", getUser, []string{token}, []string{"notification-dispatcher", "weekly-summary"}, codes.PermissionDenied},
		{"padded caller", getUser, []string{token}, []string{"notification-dispatcher "}, codes.PermissionDenied},
		{"case sensitive caller", getUser, []string{token}, []string{"Notification-Dispatcher"}, codes.PermissionDenied},
		{"unknown method", "/jobtrack.auth.v1.AuthInternalService/NewRPC", []string{token}, []string{"notification-dispatcher"}, codes.PermissionDenied},
		{"unknown service", "/jobtrack.other.v1.OtherInternalService/GetUser", []string{token}, []string{"notification-dispatcher"}, codes.PermissionDenied},
		{"public unchanged", "/jobtrack.auth.v1.AuthService/Login", nil, nil, codes.OK},
		{"health unchanged", "/grpc.health.v1.Health/Check", []string{"wrong"}, []string{"bad"}, codes.OK},
		{"allowed", getUser, []string{token}, []string{"notification-dispatcher"}, codes.OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := metadata.MD{"x-internal-token": tt.tokens, "x-caller-service": tt.callers}
			ctx := metadata.NewIncomingContext(context.Background(), md)
			called := false
			req := new(int)
			got, err := interceptor(ctx, req, &grpc.UnaryServerInfo{FullMethod: tt.method}, func(c context.Context, r any) (any, error) {
				called = true
				if c != ctx || r != req {
					t.Error("handler arguments changed")
				}
				return req, nil
			})
			if status.Code(err) != tt.code || called != (tt.code == codes.OK) {
				t.Fatalf("code=%v handler=%v", status.Code(err), called)
			}
			if called && got != req {
				t.Error("response changed")
			}
		})
	}
}

func TestInternalInterceptor_ExactRPCAllowlist(t *testing.T) {
	interceptor, _ := authctx.InternalUnaryServerInterceptor(token)
	methods := map[string]string{
		getUser: "notification-dispatcher",
		"/jobtrack.auth.v1.AuthInternalService/ListUsersByRole":                               "weekly-summary",
		"/jobtrack.analytics.v1.AnalyticsInternalService/GetInternalSummary":                  "weekly-summary",
		"/jobtrack.category_tag.v1.CategoryTagInternalService/ResolveNames":                   "job-alert",
		"/jobtrack.notification_inbox.v1.NotificationInboxInternalService/CreateNotification": "notification-dispatcher",
		"/jobtrack.job_posting.v1.JobPostingInternalService/ListJobsClosingBetween":           "deadline-reminder",
		"/jobtrack.saved_jobs.v1.SavedJobsInternalService/ListJobSavers":                      "deadline-reminder",
		"/jobtrack.resume.v1.ResumeInternalService/GetResumeFile":                             "resume-validator",
	}
	for method, allowed := range methods {
		for _, caller := range []string{"notification-dispatcher", "weekly-summary", "job-alert", "deadline-reminder", "resume-validator", "gateway", "frontend", "unknown"} {
			t.Run(method+"/"+caller, func(t *testing.T) {
				ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-internal-token", token, "x-caller-service", caller))
				_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return nil, nil })
				want := codes.PermissionDenied
				if caller == allowed {
					want = codes.OK
				}
				if status.Code(err) != want {
					t.Fatalf("got %v want %v", status.Code(err), want)
				}
			})
		}
	}
}

func TestInternalInterceptor_InvalidConfiguration(t *testing.T) {
	for _, value := range []string{"", " ", "bad token", "bad\n", "bad\x7f", "é"} {
		t.Run(value, func(t *testing.T) {
			if _, err := authctx.InternalUnaryServerInterceptor(value); err == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
}
