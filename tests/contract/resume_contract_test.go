package contract_test

import (
	"testing"

	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestResumeContract_ServicesAndHTTPRoutes(t *testing.T) {
	file := resumev1.File_jobtrack_resume_v1_resume_proto
	if file.Package() != "jobtrack.resume.v1" || file.Services().Len() != 2 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	for _, tc := range []struct {
		service string
		methods int
	}{
		{"ResumeService", 4},
		{"ResumeInternalService", 1},
	} {
		service := file.Services().ByName(protoreflect.Name(tc.service))
		if service == nil {
			t.Fatalf("missing service %s", tc.service)
		}
		if service.Methods().Len() != tc.methods {
			t.Errorf("%s has %d methods, want %d", tc.service, service.Methods().Len(), tc.methods)
		}
		if service.Methods().ByName("UpdateValidationStatus") != nil {
			t.Errorf("%s exposes an unapproved synchronous status update", tc.service)
		}
	}
	for _, tc := range []struct {
		service, method, verb, path, body string
	}{
		{"ResumeService", "UploadResume", "POST", "/v1/resumes", "*"},
		{"ResumeService", "GetResume", "GET", "/v1/resumes/{resume_id}", ""},
		{"ResumeService", "ListMyResumes", "GET", "/v1/resumes", ""},
		{"ResumeService", "DownloadResume", "GET", "/v1/resumes/{resume_id}/content", ""},
		{"ResumeInternalService", "GetResumeFile", "", "", ""},
	} {
		t.Run(tc.service+"/"+tc.method, func(t *testing.T) {
			method := file.Services().ByName(protoreflect.Name(tc.service)).Methods().ByName(protoreflect.Name(tc.method))
			if method == nil {
				t.Fatal("missing method")
			}
			if method.Input().Name() != protoreflect.Name(tc.method+"Request") || method.Output().Name() != protoreflect.Name(tc.method+"Response") {
				t.Error("method must have dedicated request and response messages")
			}
			if method.IsStreamingClient() || method.IsStreamingServer() {
				t.Error("expected unary method")
			}
			if tc.verb == "" {
				if proto.HasExtension(method.Options(), annotations.E_Http) {
					t.Error("internal method must not have an HTTP annotation")
				}
				return
			}
			if !proto.HasExtension(method.Options(), annotations.E_Http) {
				t.Fatal("public method is missing HTTP annotation")
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
				t.Errorf("unexpected HTTP rule: %v", rule)
			}
		})
	}
}

func TestResumeContract_OwnershipAndFileTransfer(t *testing.T) {
	file := resumev1.File_jobtrack_resume_v1_resume_proto
	for _, name := range []protoreflect.Name{"UploadResumeRequest", "GetResumeRequest", "ListMyResumesRequest", "DownloadResumeRequest"} {
		t.Run(string(name), func(t *testing.T) {
			fields := file.Messages().ByName(name).Fields()
			for _, identity := range []protoreflect.Name{"applicant_id", "user_id", "role"} {
				if fields.ByName(identity) != nil {
					t.Errorf("request must derive %s from caller metadata", identity)
				}
			}
		})
	}
	for _, name := range []protoreflect.Name{"UploadResumeRequest", "DownloadResumeResponse", "GetResumeFileResponse"} {
		t.Run(string(name)+"/content", func(t *testing.T) {
			content := file.Messages().ByName(name).Fields().ByName("content")
			if content == nil || content.Kind() != protoreflect.BytesKind {
				t.Error("file content must use protobuf bytes")
			}
		})
	}
	if file.Messages().ByName("Resume").Fields().ByName("content") != nil {
		t.Error("metadata responses must not contain file content")
	}
	for _, tc := range []struct {
		message, field, shared string
	}{
		{"Resume", "timestamps", "ResourceTimestamps"},
		{"ListMyResumesRequest", "pagination", "PaginationRequest"},
		{"ListMyResumesResponse", "pagination", "PaginationResponse"},
	} {
		field := file.Messages().ByName(protoreflect.Name(tc.message)).Fields().ByName(protoreflect.Name(tc.field))
		if field == nil || field.Message() == nil || field.Message().FullName() != protoreflect.FullName("jobtrack.common.v1."+tc.shared) {
			t.Errorf("%s.%s must reuse common.%s", tc.message, tc.field, tc.shared)
		}
	}
}
