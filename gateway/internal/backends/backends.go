// Package backends lists the public services the gateway exposes (AGENTS.md Section 4).
package backends

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	analyticsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/analytics/v1"
	applicantprofilev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	applicationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	applicationstatusv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	auditlogv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/audit_log/v1"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	categorytagv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/category_tag/v1"
	companyverificationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/company_verification/v1"
	employerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/employer/v1"
	interviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	jobalertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
	jobsearchv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_search/v1"
	messagingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	moderationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/moderation/v1"
	notificationinboxv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/notification_inbox/v1"
	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	reviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	savedjobsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/saved_jobs/v1"
)

// RegisterFunc matches the generated Register<Name>ServiceHandler functions.
type RegisterFunc func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

// Backend is one public service behind the gateway.
type Backend struct {
	Name     string // kebab-case folder name from AGENTS.md Section 4
	Register RegisterFunc
}

// AddrEnv is the environment variable with this service's gRPC address (AGENTS.md Section 9).
func (b Backend) AddrEnv() string {
	return strings.ToUpper(strings.ReplaceAll(b.Name, "-", "_")) + "_ADDR"
}

// All returns the 20 public services. Every *InternalService has no HTTP annotations,
// so no gateway handler is generated for it and it cannot be listed here.
func All() []Backend {
	return []Backend{
		{Name: "analytics", Register: analyticsv1.RegisterAnalyticsServiceHandler},
		{Name: "applicant-profile", Register: applicantprofilev1.RegisterApplicantProfileServiceHandler},
		{Name: "application", Register: applicationv1.RegisterApplicationServiceHandler},
		{Name: "application-status", Register: applicationstatusv1.RegisterApplicationStatusServiceHandler},
		{Name: "audit-log", Register: auditlogv1.RegisterAuditLogServiceHandler},
		{Name: "auth", Register: authv1.RegisterAuthServiceHandler},
		{Name: "category-tag", Register: categorytagv1.RegisterCategoryTagServiceHandler},
		{Name: "company-verification", Register: companyverificationv1.RegisterCompanyVerificationServiceHandler},
		{Name: "employer", Register: employerv1.RegisterEmployerServiceHandler},
		{Name: "interview", Register: interviewv1.RegisterInterviewServiceHandler},
		{Name: "job-alert", Register: jobalertv1.RegisterJobAlertServiceHandler},
		{Name: "job-posting", Register: jobpostingv1.RegisterJobPostingServiceHandler},
		{Name: "job-search", Register: jobsearchv1.RegisterJobSearchServiceHandler},
		{Name: "messaging", Register: messagingv1.RegisterMessagingServiceHandler},
		{Name: "moderation", Register: moderationv1.RegisterModerationServiceHandler},
		{Name: "notification-inbox", Register: notificationinboxv1.RegisterNotificationInboxServiceHandler},
		{Name: "offer", Register: offerv1.RegisterOfferServiceHandler},
		{Name: "resume", Register: resumev1.RegisterResumeServiceHandler},
		{Name: "review", Register: reviewv1.RegisterReviewServiceHandler},
		{Name: "saved-jobs", Register: savedjobsv1.RegisterSavedJobsServiceHandler},
	}
}
