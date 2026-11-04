package contract_test

import (
	"testing"

	applicationv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestApplicationContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := applicationv1.File_jobtrack_application_v1_application_proto
	if file.Package() != "jobtrack.application.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("ApplicationService")
	if service == nil || service.Methods().Len() != 4 {
		t.Fatal("expected only ApplicationService with four approved methods")
	}
	for _, tc := range []struct {
		method, verb, path, body string
	}{
		{"SubmitApplication", "POST", "/v1/applications", "*"},
		{"WithdrawApplication", "POST", "/v1/applications/{application_id}/withdraw", "*"},
		{"GetApplication", "GET", "/v1/applications/{application_id}", ""},
		{"ListApplications", "GET", "/v1/applications", ""},
	} {
		t.Run(tc.method, func(t *testing.T) {
			method := service.Methods().ByName(protoreflect.Name(tc.method))
			if method == nil {
				t.Fatal("missing RPC")
			}
			if method.Input().Name() != protoreflect.Name(tc.method+"Request") || method.Output().Name() != protoreflect.Name(tc.method+"Response") {
				t.Error("expected dedicated request and response messages")
			}
			if method.IsStreamingClient() || method.IsStreamingServer() {
				t.Error("expected unary RPC")
			}
			if !proto.HasExtension(method.Options(), annotations.E_Http) {
				t.Fatal("missing public HTTP binding")
			}
			rule := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
			var verb, path string
			switch pattern := rule.GetPattern().(type) {
			case *annotations.HttpRule_Get:
				verb, path = "GET", pattern.Get
			case *annotations.HttpRule_Post:
				verb, path = "POST", pattern.Post
			default:
				t.Fatalf("unexpected HTTP pattern %T", pattern)
			}
			if verb != tc.verb || path != tc.path || rule.GetBody() != tc.body || rule.GetResponseBody() != "" || len(rule.GetAdditionalBindings()) != 0 {
				t.Errorf("unexpected HTTP binding: %v", rule)
			}
		})
	}
	// Exact service/method/message sets exclude internal RPCs, status mutation
	// RPCs and local redefinitions of shared domain events.
	wantMessages := []protoreflect.Name{
		"Application", "SubmitApplicationRequest", "SubmitApplicationResponse",
		"WithdrawApplicationRequest", "WithdrawApplicationResponse",
		"GetApplicationRequest", "GetApplicationResponse",
		"ListApplicationsRequest", "ListApplicationsResponse",
	}
	if file.Messages().Len() != len(wantMessages) || file.Enums().Len() != 0 {
		t.Error("unexpected message or enum definitions")
	}
	for _, name := range wantMessages {
		if file.Messages().ByName(name) == nil {
			t.Errorf("missing message %s", name)
		}
	}
}

func TestApplicationContract_ResourceAndRequestFields(t *testing.T) {
	file := applicationv1.File_jobtrack_application_v1_application_proto
	for _, tc := range []struct {
		message string
		fields  []string
	}{
		{"Application", []string{"application_id", "applicant_id", "job_id", "resume_id", "employer_id", "withdrawn", "timestamps"}},
		{"SubmitApplicationRequest", []string{"job_id", "resume_id"}},
		{"WithdrawApplicationRequest", []string{"application_id"}},
		{"GetApplicationRequest", []string{"application_id"}},
		{"ListApplicationsRequest", []string{"pagination", "job_id"}},
		{"SubmitApplicationResponse", []string{"application"}},
		{"WithdrawApplicationResponse", []string{"application"}},
		{"GetApplicationResponse", []string{"application"}},
		{"ListApplicationsResponse", []string{"applications", "pagination"}},
	} {
		t.Run(tc.message, func(t *testing.T) {
			message := file.Messages().ByName(protoreflect.Name(tc.message))
			if message == nil {
				t.Fatal("missing message")
			}
			if message.Fields().Len() != len(tc.fields) {
				t.Errorf("unexpected field count: %d", message.Fields().Len())
			}
			for i, name := range tc.fields {
				field := message.Fields().ByName(protoreflect.Name(name))
				if field == nil || field.Number() != protoreflect.FieldNumber(i+1) {
					t.Fatalf("missing field or incorrect number: %s", name)
				}
				wantKind := protoreflect.StringKind
				var wantType protoreflect.FullName
				switch name {
				case "withdrawn":
					wantKind = protoreflect.BoolKind
				case "timestamps":
					wantKind, wantType = protoreflect.MessageKind, "jobtrack.common.v1.ResourceTimestamps"
				case "pagination":
					wantKind = protoreflect.MessageKind
					if tc.message == "ListApplicationsRequest" {
						wantType = "jobtrack.common.v1.PaginationRequest"
					} else {
						wantType = "jobtrack.common.v1.PaginationResponse"
					}
				case "application", "applications":
					wantKind, wantType = protoreflect.MessageKind, "jobtrack.application.v1.Application"
				}
				if field.Kind() != wantKind || field.IsList() != (name == "applications") {
					t.Errorf("incorrect kind/cardinality for %s", name)
				}
				if wantType != "" && (field.Message() == nil || field.Message().FullName() != wantType) {
					t.Errorf("incorrect message type for %s, want %s", name, wantType)
				}
			}
		})
	}
	for _, name := range []protoreflect.Name{"applicant_id", "employer_id"} {
		if file.Messages().ByName("SubmitApplicationRequest").Fields().ByName(name) != nil {
			t.Errorf("submission must not accept client-supplied %s", name)
		}
	}
	if file.Messages().ByName("Application").Fields().ByName("status") != nil {
		t.Error("lifecycle status belongs to Application Status")
	}
}
