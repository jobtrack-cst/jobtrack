package contract_test

import (
	"testing"

	interviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/interview/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestInterviewContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := interviewv1.File_jobtrack_interview_v1_interview_proto
	if file.Package() != "jobtrack.interview.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("InterviewService")
	if service == nil || service.Methods().Len() != 3 {
		t.Fatal("expected only InterviewService with three public methods")
	}
	for _, tc := range []struct {
		method, verb, path, body string
	}{
		{"ScheduleInterview", "POST", "/v1/applications/{application_id}/interviews", "*"},
		{"GetInterview", "GET", "/v1/interviews/{interview_id}", ""},
		{"ListInterviews", "GET", "/v1/applications/{application_id}/interviews", ""},
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
				t.Fatal("missing HTTP binding")
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
	// Exact imports exclude service dependencies and shared event redefinitions.
	wantImports := map[string]bool{
		"google/api/annotations.proto": true, "google/protobuf/timestamp.proto": true,
		"jobtrack/common/v1/common.proto": true,
	}
	if file.Imports().Len() != len(wantImports) {
		t.Error("unexpected dependency count")
	}
	for i := 0; i < file.Imports().Len(); i++ {
		if !wantImports[file.Imports().Get(i).Path()] {
			t.Errorf("unexpected dependency: %s", file.Imports().Get(i).Path())
		}
	}
}

func TestInterviewContract_ResourcesAndRequestFields(t *testing.T) {
	file := interviewv1.File_jobtrack_interview_v1_interview_proto
	cases := []struct {
		message string
		fields  []string
	}{
		{"Interview", []string{"interview_id", "application_id", "employer_id", "applicant_id", "scheduled_at", "details", "timestamps"}},
		{"ScheduleInterviewRequest", []string{"application_id", "scheduled_at", "details"}},
		{"ScheduleInterviewResponse", []string{"interview"}},
		{"GetInterviewRequest", []string{"interview_id"}},
		{"GetInterviewResponse", []string{"interview"}},
		{"ListInterviewsRequest", []string{"application_id", "pagination"}},
		{"ListInterviewsResponse", []string{"interviews", "pagination"}},
	}
	// Exact service, method and message sets exclude internal, status mutation,
	// rescheduling, cancellation, outcome and event definitions.
	if file.Messages().Len() != len(cases) || file.Enums().Len() != 0 {
		t.Error("unexpected message or enum definitions")
	}
	for _, tc := range cases {
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
					t.Fatalf("missing field or incorrect field number: %s", name)
				}
				kind := protoreflect.StringKind
				var typeName protoreflect.FullName
				switch name {
				case "scheduled_at":
					kind, typeName = protoreflect.MessageKind, "google.protobuf.Timestamp"
				case "timestamps":
					kind, typeName = protoreflect.MessageKind, "jobtrack.common.v1.ResourceTimestamps"
				case "interview", "interviews":
					kind, typeName = protoreflect.MessageKind, "jobtrack.interview.v1.Interview"
				case "pagination":
					kind = protoreflect.MessageKind
					if tc.message == "ListInterviewsRequest" {
						typeName = "jobtrack.common.v1.PaginationRequest"
					} else {
						typeName = "jobtrack.common.v1.PaginationResponse"
					}
				}
				if field.Kind() != kind || field.IsList() != (name == "interviews") {
					t.Errorf("incorrect kind/cardinality for %s", name)
				}
				if typeName != "" && (field.Message() == nil || field.Message().FullName() != typeName) {
					t.Errorf("incorrect message type for %s, want %s", name, typeName)
				}
			}
		})
	}
	for _, name := range []protoreflect.Name{"applicant_id", "employer_id"} {
		if file.Messages().ByName("ScheduleInterviewRequest").Fields().ByName(name) != nil {
			t.Errorf("schedule request must not accept client-supplied %s", name)
		}
	}
}
