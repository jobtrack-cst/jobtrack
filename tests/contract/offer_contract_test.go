package contract_test

import (
	"testing"

	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestOfferContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := offerv1.File_jobtrack_offer_v1_offer_proto
	if file.Package() != "jobtrack.offer.v1" || file.Services().Len() != 1 {
		t.Fatalf("unexpected package or service count: %s, %d", file.Package(), file.Services().Len())
	}
	service := file.Services().ByName("OfferService")
	if service == nil || service.Methods().Len() != 4 {
		t.Fatal("expected only OfferService with four public methods")
	}
	for _, tc := range []struct {
		method, verb, path, body string
	}{
		{"MakeOffer", "POST", "/v1/applications/{application_id}/offers", "*"},
		{"GetOffer", "GET", "/v1/offers/{offer_id}", ""},
		{"AcceptOffer", "POST", "/v1/offers/{offer_id}/accept", "*"},
		{"DeclineOffer", "POST", "/v1/offers/{offer_id}/decline", "*"},
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
			for _, identity := range []protoreflect.Name{"employer_id", "applicant_id"} {
				if method.Input().Fields().ByName(identity) != nil {
					t.Errorf("request must not accept client-supplied %s", identity)
				}
			}
		})
	}
	// Exact imports reject new service and event schema dependencies.
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

func TestOfferContract_ResourceAndRequestFields(t *testing.T) {
	file := offerv1.File_jobtrack_offer_v1_offer_proto
	cases := []struct {
		message string
		fields  []string
	}{
		{"Offer", []string{"offer_id", "application_id", "employer_id", "applicant_id", "terms", "status", "timestamps"}},
		{"MakeOfferRequest", []string{"application_id", "terms"}},
		{"MakeOfferResponse", []string{"offer"}},
		{"GetOfferRequest", []string{"offer_id"}},
		{"GetOfferResponse", []string{"offer"}},
		{"AcceptOfferRequest", []string{"offer_id"}},
		{"AcceptOfferResponse", []string{"offer"}},
		{"DeclineOfferRequest", []string{"offer_id"}},
		{"DeclineOfferResponse", []string{"offer"}},
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
				case "status":
					kind = protoreflect.EnumKind
				case "timestamps", "offer":
					kind = protoreflect.MessageKind
				}
				if field.Kind() != kind || field.IsList() {
					t.Fatalf("incorrect kind/cardinality for %s", name)
				}
				switch name {
				case "status":
					if field.Enum().FullName() != "jobtrack.offer.v1.OfferStatus" {
						t.Error("incorrect status enum type")
					}
				case "timestamps":
					if field.Message().FullName() != "jobtrack.common.v1.ResourceTimestamps" {
						t.Error("timestamps must use shared ResourceTimestamps")
					}
				case "offer":
					if field.Message().FullName() != "jobtrack.offer.v1.Offer" {
						t.Error("response must contain Offer")
					}
				}
			}
		})
	}
}

func TestOfferContract_StatusValues(t *testing.T) {
	file := offerv1.File_jobtrack_offer_v1_offer_proto
	enum := file.Enums().ByName("OfferStatus")
	want := []string{"OFFER_STATUS_UNSPECIFIED", "OFFER_STATUS_PENDING", "OFFER_STATUS_ACCEPTED", "OFFER_STATUS_DECLINED"}
	if file.Enums().Len() != 1 || enum == nil || enum.Values().Len() != len(want) {
		t.Fatal("expected only OfferStatus with four values")
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
