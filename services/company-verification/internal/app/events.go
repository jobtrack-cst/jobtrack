package app

import (
	"errors"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jobtrack-cst/jobtrack/services/company-verification/internal/domain"
)

// Producer is the envelope producer for every event this service publishes.
const Producer = "company-verification"

// Routing keys of the two events in events.proto (AGENTS.md Section 4).
const (
	EventEmployerVerified             = "employer.verified"
	EventEmployerVerificationRejected = "employer.verification_rejected"
)

// DecisionEvent builds the event for a decided request. occurred_at is the
// decision time, which the employer consumer uses to ignore older decisions.
func DecisionEvent(r domain.VerificationRequest, eventID string) (*eventsv1.EventEnvelope, error) {
	var routingKey string
	var data proto.Message
	switch r.Status {
	case domain.StatusApproved:
		routingKey = EventEmployerVerified
		data = &eventsv1.EmployerVerified{EmployerId: r.EmployerID}
	case domain.StatusRejected:
		routingKey = EventEmployerVerificationRejected
		data = &eventsv1.EmployerVerificationRejected{EmployerId: r.EmployerID, Reason: r.RejectionReason}
	default:
		return nil, errors.New("only an approved or rejected request has an event")
	}
	payload, err := anypb.New(data)
	if err != nil {
		return nil, errors.New("encode decision event failed")
	}
	return &eventsv1.EventEnvelope{
		EventId:    eventID,
		Type:       routingKey,
		OccurredAt: timestamppb.New(r.UpdatedAt),
		Producer:   Producer,
		Data:       payload,
	}, nil
}
