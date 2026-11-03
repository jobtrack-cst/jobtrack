package contract_test

import (
	"testing"

	reviewv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/review/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestReviewContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := reviewv1.File_jobtrack_review_v1_review_proto
	if file.Package() != "jobtrack.review.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("ReviewService")
	if service == nil || service.Methods().Len() != 2 {
		t.Fatal("expected only ReviewService with two public methods, no internal or status mutation RPCs")
	}
	for _, tc := range []struct {
		method, verb, path, body string
	}{
		{"CompleteReview", "POST", "/v1/applications/{application_id}/reviews", "*"},
		{"GetReview", "GET", "/v1/applications/{application_id}/review", ""},
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

func TestReviewContract_ResourceAndRequestFields(t *testing.T) {
	file := reviewv1.File_jobtrack_review_v1_review_proto
	cases := []struct {
		message string
		fields  []string
	}{
		{"Review", []string{"review_id", "application_id", "employer_id", "score", "decision", "timestamps"}},
		{"CompleteReviewRequest", []string{"application_id", "score", "decision"}},
		{"CompleteReviewResponse", []string{"review"}},
		{"GetReviewRequest", []string{"application_id"}},
		{"GetReviewResponse", []string{"review"}},
	}
	if file.Messages().Len() != len(cases) {
		t.Error("unexpected message definitions, including possible local events")
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
				switch name {
				case "score":
					kind = protoreflect.Int32Kind
				case "decision":
					kind = protoreflect.EnumKind
				case "timestamps", "review":
					kind = protoreflect.MessageKind
				}
				if field.Kind() != kind || field.IsList() {
					t.Fatalf("incorrect kind/cardinality for %s", name)
				}
				switch name {
				case "decision":
					if field.Enum().FullName() != "jobtrack.review.v1.ReviewDecision" {
						t.Error("incorrect decision enum type")
					}
				case "timestamps":
					if field.Message().FullName() != "jobtrack.common.v1.ResourceTimestamps" {
						t.Error("timestamps must use shared ResourceTimestamps")
					}
				case "review":
					if field.Message().FullName() != "jobtrack.review.v1.Review" {
						t.Error("response must contain Review")
					}
				}
			}
		})
	}
	if file.Messages().ByName("CompleteReviewRequest").Fields().ByName("employer_id") != nil {
		t.Error("employer identity must come from metadata, not the completion request")
	}
}

func TestReviewContract_DecisionValues(t *testing.T) {
	file := reviewv1.File_jobtrack_review_v1_review_proto
	enum := file.Enums().ByName("ReviewDecision")
	want := []string{"REVIEW_DECISION_UNSPECIFIED", "REVIEW_DECISION_SHORTLISTED", "REVIEW_DECISION_REJECTED"}
	if file.Enums().Len() != 1 || enum == nil || enum.Values().Len() != len(want) {
		t.Fatal("expected only ReviewDecision with three values")
	}
	for number, name := range want {
		t.Run(name, func(t *testing.T) {
			value := enum.Values().ByName(protoreflect.Name(name))
			if value == nil || value.Number() != protoreflect.EnumNumber(number) {
				t.Errorf("expected %s = %d", name, number)
			}
		})
	}
}
