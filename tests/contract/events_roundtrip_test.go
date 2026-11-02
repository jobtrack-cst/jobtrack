package contract_test

import (
	"testing"
	"time"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEventEnvelope_JSONRoundTrip_PreservesPayloadAndMetadata(t *testing.T) {
	const (
		applicationID = "11111111-1111-4111-8111-111111111111"
		applicantID   = "22222222-2222-4222-8222-222222222222"
		jobID         = "33333333-3333-4333-8333-333333333333"
		resumeID      = "44444444-4444-4444-8444-444444444444"
		employerID    = "55555555-5555-4555-8555-555555555555"
	)
	occurredAt := timestamppb.New(time.Date(2026, time.November, 3, 6, 30, 0, 123456789, time.UTC))
	tests := []struct {
		name       string
		routingKey string
		producer   string
		event      proto.Message
	}{
		{
			name: "JobPublished", routingKey: "job.published", producer: "job-posting",
			event: &eventsv1.JobPublished{
				JobId: jobID, EmployerId: employerID, Title: "Software Engineering Intern",
				CategoryId: "66666666-6666-4666-8666-666666666666",
				TagIds:     []string{"77777777-7777-4777-8777-777777777777"}, Location: "Thimphu",
				Deadline: timestamppb.New(time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "ApplicationSubmitted", routingKey: "application.submitted", producer: "application",
			event: &eventsv1.ApplicationSubmitted{
				ApplicationId: applicationID, ApplicantId: applicantID, JobId: jobID,
				ResumeId: resumeID, EmployerId: employerID,
			},
		},
		{
			name: "StatusChanged", routingKey: "application.status_changed", producer: "application-status",
			event: &eventsv1.StatusChanged{
				ApplicationId: applicationID, ApplicantId: applicantID,
				PreviousStatus: "SUBMITTED", NewStatus: "UNDER_REVIEW",
			},
		},
		{
			name: "ResumeUploaded", routingKey: "resume.uploaded", producer: "resume",
			event: &eventsv1.ResumeUploaded{ResumeId: resumeID, ApplicantId: applicantID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := anypb.New(tt.event)
			if err != nil {
				t.Fatalf("pack payload: %v", err)
			}
			original := &eventsv1.EventEnvelope{
				EventId: "88888888-8888-4888-8888-888888888888", Type: tt.routingKey,
				OccurredAt: occurredAt, Producer: tt.producer, Data: payload,
			}
			encoded, err := protojson.Marshal(original)
			if err != nil {
				t.Fatalf("marshal envelope: %v", err)
			}
			decoded := new(eventsv1.EventEnvelope)
			if err := protojson.Unmarshal(encoded, decoded); err != nil {
				t.Fatalf("unmarshal envelope: %v", err)
			}
			if decoded.GetEventId() != original.GetEventId() || decoded.GetType() != tt.routingKey || decoded.GetProducer() != tt.producer {
				t.Errorf("envelope metadata changed: got %v", decoded)
			}
			if !proto.Equal(decoded.GetOccurredAt(), occurredAt) {
				t.Errorf("occurred_at changed: got %v, want %v", decoded.GetOccurredAt(), occurredAt)
			}
			if decoded.GetData() == nil {
				t.Fatal("decoded envelope has no payload")
			}
			unpacked, err := decoded.GetData().UnmarshalNew()
			if err != nil {
				t.Fatalf("unpack payload: %v", err)
			}
			if !proto.Equal(unpacked, tt.event) {
				t.Errorf("payload changed: got %v, want %v", unpacked, tt.event)
			}
		})
	}
}
