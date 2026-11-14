package app

import (
	"errors"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OfferEvent uses the committed offer's time and participants.
func OfferEvent(o domain.Offer, eventID string) (*eventsv1.EventEnvelope, error) {
	var key string
	var data proto.Message
	switch o.Status {
	case domain.OfferStatusPending:
		key = "offer.made"
		data = &eventsv1.OfferMade{OfferId: o.ID, ApplicationId: o.ApplicationID, ApplicantId: o.ApplicantID}
	case domain.OfferStatusAccepted:
		key = "offer.accepted"
		data = &eventsv1.OfferAccepted{OfferId: o.ID, ApplicationId: o.ApplicationID, ApplicantId: o.ApplicantID}
	case domain.OfferStatusDeclined:
		key = "offer.declined"
		data = &eventsv1.OfferDeclined{OfferId: o.ID, ApplicationId: o.ApplicationID, ApplicantId: o.ApplicantID}
	default:
		return nil, errors.New("invalid offer event status")
	}
	payload, err := anypb.New(data)
	if err != nil {
		return nil, err
	}
	return &eventsv1.EventEnvelope{EventId: eventID, Type: key, Producer: "offer", OccurredAt: timestamppb.New(o.UpdatedAt.UTC()), Data: payload}, nil
}
