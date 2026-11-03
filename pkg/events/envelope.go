package events

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// MarshalEnvelope validates and encodes the shared envelope using protobuf JSON.
// Type is the caller-supplied RabbitMQ routing key and is preserved unchanged.
func MarshalEnvelope(envelope *eventsv1.EventEnvelope) ([]byte, error) {
	if err := validateEnvelope(envelope); err != nil {
		return nil, err
	}
	data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal event envelope: %w", err)
	}
	return data, nil
}

// UnmarshalEnvelope decodes and validates the shared envelope and its typed data.
func UnmarshalEnvelope(data []byte) (*eventsv1.EventEnvelope, error) {
	envelope := new(eventsv1.EventEnvelope)
	if err := protojson.Unmarshal(data, envelope); err != nil {
		return nil, fmt.Errorf("unmarshal event envelope: %w", err)
	}
	if err := validateEnvelope(envelope); err != nil {
		return nil, err
	}
	return envelope, nil
}

func validateEnvelope(envelope *eventsv1.EventEnvelope) error {
	if envelope == nil {
		return errors.New("event envelope is required")
	}
	if !uuidPattern.MatchString(envelope.GetEventId()) {
		return errors.New("event_id must be a UUID string")
	}
	if strings.TrimSpace(envelope.GetType()) == "" {
		return errors.New("type (RabbitMQ routing key) is required")
	}
	if envelope.GetOccurredAt() == nil {
		return errors.New("occurred_at is required")
	}
	if err := envelope.GetOccurredAt().CheckValid(); err != nil {
		return fmt.Errorf("invalid occurred_at: %w", err)
	}
	if strings.TrimSpace(envelope.GetProducer()) == "" {
		return errors.New("producer is required")
	}
	if envelope.GetData() == nil {
		return errors.New("data is required")
	}
	if _, err := envelope.GetData().UnmarshalNew(); err != nil {
		return fmt.Errorf("invalid Any data: %w", err)
	}
	return nil
}
