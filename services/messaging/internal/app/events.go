package app

import (
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MessageEvent contains only the approved notification payload.
func MessageEvent(m domain.Message, eventID string) (*eventsv1.EventEnvelope, error) {
	data, err := anypb.New(&eventsv1.MessageSent{MessageId: m.ID, ApplicationId: m.ApplicationID, RecipientId: m.RecipientID})
	if err != nil {
		return nil, err
	}
	return &eventsv1.EventEnvelope{EventId: eventID, Type: "message.sent", Producer: "messaging", OccurredAt: timestamppb.New(m.CreatedAt.UTC()), Data: data}, nil
}
