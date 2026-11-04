package contract_test

import (
	"testing"

	applicantprofilev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/applicant_profile/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestApplicantProfileContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := applicantprofilev1.File_jobtrack_applicant_profile_v1_applicant_profile_proto
	if file.Package() != "jobtrack.applicant_profile.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("ApplicantProfileService")
	if service == nil || service.Methods().Len() != 3 {
		t.Fatal("expected only ApplicantProfileService with three public RPCs")
	}
	for _, tc := range []struct {
		method, verb, body string
	}{
		{"CreateMyProfile", "POST", "*"},
		{"GetMyProfile", "GET", ""},
		{"UpdateMyProfile", "PUT", "*"},
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
			case *annotations.HttpRule_Put:
				verb, path = "PUT", pattern.Put
			default:
				t.Fatalf("unexpected HTTP pattern %T", pattern)
			}
			if verb != tc.verb || path != "/v1/applicant/profile" || rule.GetBody() != tc.body || rule.GetResponseBody() != "" || len(rule.GetAdditionalBindings()) != 0 {
				t.Errorf("unexpected HTTP binding: %v", rule)
			}
		})
	}
	// An exact import set excludes new service or event schema dependencies.
	wantImports := map[string]bool{"google/api/annotations.proto": true, "jobtrack/common/v1/common.proto": true}
	if file.Imports().Len() != len(wantImports) {
		t.Error("unexpected dependency count")
	}
	for i := 0; i < file.Imports().Len(); i++ {
		if !wantImports[file.Imports().Get(i).Path()] {
			t.Errorf("unexpected dependency: %s", file.Imports().Get(i).Path())
		}
	}
}

func TestApplicantProfileContract_ResourcesAndEditableFields(t *testing.T) {
	file := applicantprofilev1.File_jobtrack_applicant_profile_v1_applicant_profile_proto
	cases := []struct {
		message string
		fields  []string
	}{
		{"ApplicantProfile", []string{"applicant_id", "summary", "education", "skills", "timestamps"}},
		{"Education", []string{"institution", "qualification", "start_year", "end_year"}},
		{"CreateMyProfileRequest", []string{"summary", "education", "skills"}},
		{"GetMyProfileRequest", nil},
		{"UpdateMyProfileRequest", []string{"summary", "education", "skills"}},
		{"CreateMyProfileResponse", []string{"profile"}},
		{"GetMyProfileResponse", []string{"profile"}},
		{"UpdateMyProfileResponse", []string{"profile"}},
	}
	// Exact message and enum counts reject unintended events or extra resources.
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
				case "start_year", "end_year":
					kind = protoreflect.Int32Kind
				case "education":
					kind, typeName = protoreflect.MessageKind, "jobtrack.applicant_profile.v1.Education"
				case "timestamps":
					kind, typeName = protoreflect.MessageKind, "jobtrack.common.v1.ResourceTimestamps"
				case "profile":
					kind, typeName = protoreflect.MessageKind, "jobtrack.applicant_profile.v1.ApplicantProfile"
				}
				if field.Kind() != kind || field.IsList() != (name == "education" || name == "skills") {
					t.Errorf("incorrect kind/cardinality for %s", name)
				}
				if typeName != "" && (field.Message() == nil || field.Message().FullName() != typeName) {
					t.Errorf("incorrect message type for %s, want %s", name, typeName)
				}
			}
		})
	}
	for _, name := range []protoreflect.Name{"CreateMyProfileRequest", "UpdateMyProfileRequest"} {
		if file.Messages().ByName(name).Fields().ByName("applicant_id") != nil {
			t.Errorf("%s must derive applicant identity from metadata", name)
		}
	}
}
