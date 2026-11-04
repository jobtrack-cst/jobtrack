package contract_test

import (
	"testing"

	applicationstatusv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/application_status/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestApplicationStatusContract_ReadOnlyServiceAndHTTPRoutes(t *testing.T) {
	file := applicationstatusv1.File_jobtrack_application_status_v1_application_status_proto
	if file.Package() != "jobtrack.application_status.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("ApplicationStatusService")
	if service == nil || service.Methods().Len() != 2 {
		t.Fatal("expected only ApplicationStatusService with two read-only methods; no internal or mutation RPCs")
	}
	for _, tc := range []struct {
		method, path string
	}{
		{"GetApplicationStatus", "/v1/applications/{application_id}/status"},
		{"ListApplicationStatusHistory", "/v1/applications/{application_id}/status-history"},
	} {
		t.Run(tc.method, func(t *testing.T) {
			method := service.Methods().ByName(protoreflect.Name(tc.method))
			if method == nil {
				t.Fatal("missing read-only method")
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
			pattern, ok := rule.GetPattern().(*annotations.HttpRule_Get)
			if !ok || pattern.Get != tc.path || rule.GetBody() != "" || rule.GetResponseBody() != "" || len(rule.GetAdditionalBindings()) != 0 {
				t.Errorf("expected only GET %s, got %v", tc.path, rule)
			}
		})
	}
	// An exact message allowlist rejects accidental event or mutation definitions.
	wantMessages := []protoreflect.Name{
		"ApplicationStatusRecord", "StatusHistoryEntry",
		"GetApplicationStatusRequest", "GetApplicationStatusResponse",
		"ListApplicationStatusHistoryRequest", "ListApplicationStatusHistoryResponse",
	}
	if file.Messages().Len() != len(wantMessages) {
		t.Errorf("unexpected message count: %d", file.Messages().Len())
	}
	for _, name := range wantMessages {
		if file.Messages().ByName(name) == nil {
			t.Errorf("missing message %s", name)
		}
	}
}

func TestApplicationStatusContract_LifecycleValues(t *testing.T) {
	file := applicationstatusv1.File_jobtrack_application_status_v1_application_status_proto
	enum := file.Enums().ByName("ApplicationStatus")
	want := []string{"UNSPECIFIED", "SUBMITTED", "UNDER_REVIEW", "SHORTLISTED", "INTERVIEW", "OFFERED", "ACCEPTED", "DECLINED", "REJECTED", "WITHDRAWN"}
	if file.Enums().Len() != 1 || enum == nil || enum.Values().Len() != len(want) {
		t.Fatal("expected only the documented lifecycle enum and its ten values")
	}
	for number, suffix := range want {
		t.Run(suffix, func(t *testing.T) {
			value := enum.Values().ByName(protoreflect.Name("APPLICATION_STATUS_" + suffix))
			if value == nil || value.Number() != protoreflect.EnumNumber(number) {
				t.Errorf("expected APPLICATION_STATUS_%s = %d", suffix, number)
			}
		})
	}
}

func TestApplicationStatusContract_ResourcesAndSharedTypes(t *testing.T) {
	file := applicationstatusv1.File_jobtrack_application_status_v1_application_status_proto
	for _, tc := range []struct {
		message, field, typeName string
		kind                     protoreflect.Kind
		repeated                 bool
	}{
		{"ApplicationStatusRecord", "application_id", "", protoreflect.StringKind, false},
		{"ApplicationStatusRecord", "status", "jobtrack.application_status.v1.ApplicationStatus", protoreflect.EnumKind, false},
		{"ApplicationStatusRecord", "timestamps", "jobtrack.common.v1.ResourceTimestamps", protoreflect.MessageKind, false},
		{"StatusHistoryEntry", "previous_status", "jobtrack.application_status.v1.ApplicationStatus", protoreflect.EnumKind, false},
		{"StatusHistoryEntry", "new_status", "jobtrack.application_status.v1.ApplicationStatus", protoreflect.EnumKind, false},
		{"StatusHistoryEntry", "occurred_at", "google.protobuf.Timestamp", protoreflect.MessageKind, false},
		{"GetApplicationStatusRequest", "application_id", "", protoreflect.StringKind, false},
		{"GetApplicationStatusResponse", "status_record", "jobtrack.application_status.v1.ApplicationStatusRecord", protoreflect.MessageKind, false},
		{"ListApplicationStatusHistoryRequest", "application_id", "", protoreflect.StringKind, false},
		{"ListApplicationStatusHistoryRequest", "pagination", "jobtrack.common.v1.PaginationRequest", protoreflect.MessageKind, false},
		{"ListApplicationStatusHistoryResponse", "entries", "jobtrack.application_status.v1.StatusHistoryEntry", protoreflect.MessageKind, true},
		{"ListApplicationStatusHistoryResponse", "pagination", "jobtrack.common.v1.PaginationResponse", protoreflect.MessageKind, false},
	} {
		t.Run(tc.message+"/"+tc.field, func(t *testing.T) {
			message := file.Messages().ByName(protoreflect.Name(tc.message))
			if message == nil {
				t.Fatal("missing message")
			}
			field := message.Fields().ByName(protoreflect.Name(tc.field))
			if field == nil || field.Kind() != tc.kind || field.IsList() != tc.repeated {
				t.Fatal("missing field or incorrect kind/cardinality")
			}
			if tc.kind == protoreflect.MessageKind && field.Message().FullName() != protoreflect.FullName(tc.typeName) {
				t.Errorf("incorrect message type: %s", field.Message().FullName())
			}
			if tc.kind == protoreflect.EnumKind && field.Enum().FullName() != protoreflect.FullName(tc.typeName) {
				t.Errorf("incorrect enum type: %s", field.Enum().FullName())
			}
		})
	}
}
