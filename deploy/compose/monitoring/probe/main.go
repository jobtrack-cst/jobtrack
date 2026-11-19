// Command probe issues only read-only public RPCs on the local Compose network.
// It uses empty requests without identity to exercise failures, plus one synthetic
// category lookup to exercise success. It never prints responses or credentials.
package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/audit_log/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/company_verification/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_search/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/moderation/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/notification_inbox/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	_ "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	targets := []struct{ name, service, method string }{
		{"analytics", "jobtrack.analytics.v1.AnalyticsService", "GetSummary"},
		{"applicant-profile", "jobtrack.applicant_profile.v1.ApplicantProfileService", "GetMyProfile"},
		{"application", "jobtrack.application.v1.ApplicationService", "GetApplication"},
		{"application-status", "jobtrack.application_status.v1.ApplicationStatusService", "GetApplicationStatus"},
		{"audit-log", "jobtrack.audit_log.v1.AuditLogService", "ListAuditEntries"},
		{"auth", "jobtrack.auth.v1.AuthService", "GetCurrentUser"},
		{"category-tag", "jobtrack.category_tag.v1.CategoryTagService", "ListCategories"},
		{"company-verification", "jobtrack.company_verification.v1.CompanyVerificationService", "ListMyVerificationRequests"},
		{"employer", "jobtrack.employer.v1.EmployerService", "GetMyCompanyProfile"},
		{"interview", "jobtrack.interview.v1.InterviewService", "GetInterview"},
		{"job-alert", "jobtrack.job_alert.v1.JobAlertService", "ListMyJobAlerts"},
		{"job-posting", "jobtrack.job_posting.v1.JobPostingService", "GetJob"},
		{"job-search", "jobtrack.job_search.v1.JobSearchService", "SearchJobs"},
		{"messaging", "jobtrack.messaging.v1.MessagingService", "ListMessages"},
		{"moderation", "jobtrack.moderation.v1.ModerationService", "ListReports"},
		{"notification-inbox", "jobtrack.notification_inbox.v1.NotificationInboxService", "ListMyNotifications"},
		{"offer", "jobtrack.offer.v1.OfferService", "GetOffer"},
		{"resume", "jobtrack.resume.v1.ResumeService", "GetResume"},
		{"review", "jobtrack.review.v1.ReviewService", "GetReview"},
		{"saved-jobs", "jobtrack.saved_jobs.v1.SavedJobsService", "ListMySavedJobs"},
	}
	for _, target := range targets {
		descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(target.service + "." + target.method))
		if err != nil {
			panic(err)
		}
		method := descriptor.(protoreflect.MethodDescriptor)
		conn, err := grpc.NewClient(target.name+":9090", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
		if err != nil {
			panic(err)
		}
		code := codes.OK
		for i := 0; i < 4; i++ {
			err = conn.Invoke(context.Background(), "/"+target.service+"/"+target.method, dynamicpb.NewMessage(method.Input()), dynamicpb.NewMessage(method.Output()))
			code = status.Code(err)
			if code == codes.Unavailable || code == codes.Unimplemented || code == codes.DeadlineExceeded {
				fmt.Println("FAIL:", target.name, code)
				os.Exit(1)
			}
		}
		fmt.Println("PASS: read-only probe", target.name, code)
		if target.name == "category-tag" {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "11111111-1111-4111-8111-111111111111", "x-user-role", "applicant"))
			err = conn.Invoke(ctx, "/"+target.service+"/"+target.method, dynamicpb.NewMessage(method.Input()), dynamicpb.NewMessage(method.Output()))
			if err != nil {
				fmt.Println("FAIL: category success", status.Code(err))
				os.Exit(1)
			}
			fmt.Println("PASS: synthetic category read OK")
		}
		conn.Close()
	}
}
