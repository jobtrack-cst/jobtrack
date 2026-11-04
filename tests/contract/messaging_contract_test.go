package contract_test

import (
	"testing"

	messagingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestMessagingContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := messagingv1.File_jobtrack_messaging_v1_messaging_proto
	if file.Package() != "jobtrack.messaging.v1" || file.Services().Len() != 1 {
		t.Fatal("expected messaging package with only one public service")
	}
	service := file.Services().ByName("MessagingService")
	if service == nil || service.Methods().Len() != 2 {
		t.Fatal("expected only SendMessage and ListMessages, without internal RPCs")
	}
	for _, tc := range []struct{ method, verb, body string }{
		{"SendMessage", "POST", "*"},
		{"ListMessages", "GET", ""},
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
			if method.Input().Fields().ByName("sender_id") != nil {
				t.Error("clients must not supply sender identity")
			}
			if tc.method == "SendMessage" && method.Input().Fields().ByName("recipient_id") != nil {
				t.Error("clients must not supply recipient identity")
			}
			if !proto.HasExtension(method.Options(), annotations.E_Http) {
				t.Fatal("missing public HTTP annotation")
			}
			rule := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
			var verb, path string
			switch pattern := rule.GetPattern().(type) {
			case *annotations.HttpRule_Post:
				verb, path = "POST", pattern.Post
			case *annotations.HttpRule_Get:
				verb, path = "GET", pattern.Get
			default:
				t.Fatalf("unexpected HTTP pattern %T", pattern)
			}
			if verb != tc.verb || path != "/v1/applications/{application_id}/messages" || rule.GetBody() != tc.body || rule.GetResponseBody() != "" || len(rule.GetAdditionalBindings()) != 0 {
				t.Errorf("unexpected HTTP binding: %v", rule)
			}
		})
	}
	// Exact imports prevent new service or event-schema dependencies.
	wantImports := map[string]bool{"google/api/annotations.proto": true, "jobtrack/common/v1/common.proto": true}
	if file.Imports().Len() != len(wantImports) {
		t.Error("unexpected dependency count")
	}
	for i := 0; i < file.Imports().Len(); i++ {
		if !wantImports[file.Imports().Get(i).Path()] {
			t.Errorf("unapproved dependency: %s", file.Imports().Get(i).Path())
		}
	}
}

func TestMessagingContract_FieldsAndPagination(t *testing.T) {
	type fieldSpec struct {
		name        string
		messageType protoreflect.FullName
		repeated    bool
	}
	cases := []struct {
		message string
		fields  []fieldSpec
	}{
		{"Message", []fieldSpec{
			{"message_id", "", false}, {"application_id", "", false},
			{"sender_id", "", false}, {"recipient_id", "", false},
			{"content", "", false}, {"timestamps", "jobtrack.common.v1.ResourceTimestamps", false},
		}},
		{"SendMessageRequest", []fieldSpec{{"application_id", "", false}, {"content", "", false}}},
		{"SendMessageResponse", []fieldSpec{{"message", "jobtrack.messaging.v1.Message", false}}},
		{"ListMessagesRequest", []fieldSpec{{"application_id", "", false}, {"pagination", "jobtrack.common.v1.PaginationRequest", false}}},
		{"ListMessagesResponse", []fieldSpec{{"messages", "jobtrack.messaging.v1.Message", true}, {"pagination", "jobtrack.common.v1.PaginationResponse", false}}},
	}
	file := messagingv1.File_jobtrack_messaging_v1_messaging_proto
	if file.Messages().Len() != len(cases) || file.Enums().Len() != 0 || file.Extensions().Len() != 0 {
		t.Error("unexpected extra definitions, including possible local events")
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
			for i, want := range tc.fields {
				field := message.Fields().ByName(protoreflect.Name(want.name))
				number := protoreflect.FieldNumber(i + 1)
				if tc.message == "SendMessageRequest" && want.name == "content" {
					number = 3
				}
				if field == nil || field.Number() != number {
					t.Fatalf("missing or incorrectly numbered field %s", want.name)
				}
				kind := protoreflect.StringKind
				if want.messageType != "" {
					kind = protoreflect.MessageKind
				}
				if field.Kind() != kind || field.IsList() != want.repeated || field.IsMap() {
					t.Fatalf("wrong type/cardinality for %s", want.name)
				}
				if want.messageType != "" && field.Message().FullName() != want.messageType {
					t.Errorf("wrong message type for %s: %s", want.name, field.Message().FullName())
				}
			}
			if tc.message == "SendMessageRequest" && (!message.ReservedNames().Has("recipient_id") || !message.ReservedRanges().Has(2)) {
				t.Error("removed recipient field name and number must remain reserved")
			}
		})
	}
}
