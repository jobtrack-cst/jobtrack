package events_test

import (
	"encoding/json"
	"strings"
	"testing"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	eventID = "11111111-1111-4111-8111-111111111111"
	userID  = "22222222-2222-4222-8222-222222222222"
	jobID   = "33333333-3333-4333-8333-333333333333"
)

func envelopeFor(t *testing.T, event proto.Message, routingKey string) *eventsv1.EventEnvelope {
	t.Helper()
	data, err := anypb.New(event)
	if err != nil {
		t.Fatal(err)
	}
	return &eventsv1.EventEnvelope{
		EventId: eventID, Type: routingKey, Producer: "test-producer",
		OccurredAt: &timestamppb.Timestamp{Seconds: 1793685600, Nanos: 123456789}, Data: data,
	}
}

func TestEnvelope_JSONRoundTrip_PreservesMetadataAndTypedData(t *testing.T) {
	tests := []struct {
		routingKey string
		event      proto.Message
	}{
		{"job.published", &eventsv1.JobPublished{
			JobId: jobID, EmployerId: userID, Title: "Intern \"Go\"\nDeveloper", CategoryId: eventID,
			TagIds: []string{eventID}, Location: "Thimphu", Deadline: &timestamppb.Timestamp{Seconds: 1794290400},
		}},
		{"application.submitted", &eventsv1.ApplicationSubmitted{
			ApplicationId: eventID, ApplicantId: userID, JobId: jobID, ResumeId: eventID, EmployerId: userID,
		}},
		{"application.status_changed", &eventsv1.StatusChanged{
			ApplicationId: eventID, ApplicantId: userID, PreviousStatus: "SUBMITTED", NewStatus: "UNDER_REVIEW",
		}},
		{"resume.uploaded", &eventsv1.ResumeUploaded{ResumeId: eventID, ApplicantId: userID}},
		{"offer.accepted", &eventsv1.OfferAccepted{OfferId: eventID, ApplicationId: eventID, ApplicantId: userID}},
		{"deadline.reminder", &eventsv1.DeadlineReminder{
			JobId: jobID, ApplicantId: userID, Deadline: &timestamppb.Timestamp{Seconds: 1794290400},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.routingKey, func(t *testing.T) {
			original := envelopeFor(t, tt.event, tt.routingKey)
			encoded, err := events.MarshalEnvelope(original)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(encoded) {
				t.Fatalf("output is not valid JSON: %s", encoded)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"event_id", "type", "occurred_at", "producer", "data"} {
				if _, ok := fields[field]; !ok {
					t.Errorf("missing JSON envelope field %s", field)
				}
			}
			decoded, err := events.UnmarshalEnvelope(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(decoded, original) {
				t.Errorf("envelope changed: got %v, want %v", decoded, original)
			}
			payload, err := decoded.GetData().UnmarshalNew()
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(payload, tt.event) {
				t.Errorf("typed payload changed: got %v, want %v", payload, tt.event)
			}
		})
	}
}

func TestEnvelope_RejectsMissingMetadata(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*eventsv1.EventEnvelope)
		field  string
	}{
		{"event ID", func(e *eventsv1.EventEnvelope) { e.EventId = "" }, "event_id"},
		{"type", func(e *eventsv1.EventEnvelope) { e.Type = "" }, "type"},
		{"whitespace type", func(e *eventsv1.EventEnvelope) { e.Type = "  " }, "type"},
		{"timestamp", func(e *eventsv1.EventEnvelope) { e.OccurredAt = nil }, "occurred_at"},
		{"producer", func(e *eventsv1.EventEnvelope) { e.Producer = "" }, "producer"},
		{"whitespace producer", func(e *eventsv1.EventEnvelope) { e.Producer = "  " }, "producer"},
		{"data", func(e *eventsv1.EventEnvelope) { e.Data = nil }, "data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := envelopeFor(t, &eventsv1.ResumeUploaded{ResumeId: eventID, ApplicantId: userID}, "resume.uploaded")
			tt.remove(envelope)
			encoded, err := events.MarshalEnvelope(envelope)
			if err == nil || encoded != nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s validation error and nil output, got %v", tt.field, err)
			}
			// Bypass the helper to exercise validation of externally received JSON.
			raw, err := protojson.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := events.UnmarshalEnvelope(raw)
			if err == nil || decoded != nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s validation error and nil envelope, got %v", tt.field, err)
			}
		})
	}
}

func TestMarshalEnvelope_RejectsMalformedEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*eventsv1.EventEnvelope)
		field  string
	}{
		{"invalid UUID", func(e *eventsv1.EventEnvelope) { e.EventId = "not-a-uuid" }, "event_id"},
		{"timestamp too late", func(e *eventsv1.EventEnvelope) { e.OccurredAt.Seconds = 253402300800 }, "occurred_at"},
		{"timestamp too early", func(e *eventsv1.EventEnvelope) { e.OccurredAt.Seconds = -62135596801 }, "occurred_at"},
		{"negative nanos", func(e *eventsv1.EventEnvelope) { e.OccurredAt.Nanos = -1 }, "occurred_at"},
		{"nanos overflow", func(e *eventsv1.EventEnvelope) { e.OccurredAt.Nanos = 1000000000 }, "occurred_at"},
		{"empty Any", func(e *eventsv1.EventEnvelope) { e.Data = &anypb.Any{} }, "Any data"},
		{"corrupt Any", func(e *eventsv1.EventEnvelope) { e.Data.Value = []byte{0xff} }, "Any data"},
		{"unknown Any type", func(e *eventsv1.EventEnvelope) { e.Data.TypeUrl = "type.googleapis.com/unknown.Event" }, "Any data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := envelopeFor(t, &eventsv1.ResumeUploaded{ResumeId: eventID, ApplicantId: userID}, "resume.uploaded")
			tt.mutate(envelope)
			encoded, err := events.MarshalEnvelope(envelope)
			if err == nil || encoded != nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s validation error and nil output, got %v", tt.field, err)
			}
		})
	}
	if data, err := events.MarshalEnvelope(nil); err == nil || data != nil {
		t.Fatal("expected nil envelope rejection")
	}
}

func TestUnmarshalEnvelope_RejectsMalformedJSONAndTimestamps(t *testing.T) {
	valid := envelopeFor(t, &eventsv1.ResumeUploaded{ResumeId: eventID, ApplicantId: userID}, "resume.uploaded")
	raw, err := events.MarshalEnvelope(valid)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data string
	}{
		{"empty input", ""},
		{"invalid JSON", "{"},
		{"JSON array", "[]"},
		{"empty object", "{}"},
		{"invalid UUID", strings.Replace(string(raw), eventID, "invalid", 1)},
		{"invalid timestamp", strings.Replace(string(raw), valid.OccurredAt.AsTime().Format("2006-01-02T15:04:05.999999999Z07:00"), "not-a-timestamp", 1)},
		{"out of range timestamp", strings.Replace(string(raw), valid.OccurredAt.AsTime().Format("2006-01-02T15:04:05.999999999Z07:00"), "10000-01-01T00:00:00Z", 1)},
		{"unknown Any type", strings.Replace(string(raw), "jobtrack.events.v1.ResumeUploaded", "unknown.Event", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := events.UnmarshalEnvelope([]byte(tt.data))
			if err == nil || decoded != nil {
				t.Fatalf("expected malformed envelope rejection, got %v", decoded)
			}
		})
	}
}
