package domain

import (
	"bytes"
	"encoding/json"
	"regexp"
	"time"
	"unicode/utf8"
)

// Field limits noted in the JT-35 PR and used for boundary value tests.
// event_type and producer are ASCII by their patterns, so bytes equal
// characters. data_json is limited in bytes because it can hold any text.
const (
	MaxEventTypeLength = 100
	MaxProducerLength  = 50
	MaxDataJSONBytes   = 65536
)

var (
	// Routing keys such as "job_alert.matched": at least two parts.
	eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	// Same rule as scripts/new-service.sh uses for service names.
	producerPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// Entry is one recorded domain event. There is no actor yet because the
// envelope has no actor_id (AGENTS.md Section 16).
type Entry struct {
	EventID    string
	EventType  string
	Producer   string
	OccurredAt time.Time
	// Stored and returned exactly as received, so it is TEXT, not JSONB.
	DataJSON string
}

// ValidateEventType is shared by NewEntry and the list filter. Values are
// not trimmed: a routing key with spaces is wrong, not untidy.
func ValidateEventType(s string) error {
	if len(s) > MaxEventTypeLength || !eventTypePattern.MatchString(s) {
		return ErrInvalidEventType
	}
	return nil
}

func validateProducer(s string) error {
	if len(s) > MaxProducerLength || !producerPattern.MatchString(s) {
		return ErrInvalidProducer
	}
	return nil
}

// validateDataJSON needs the UTF-8 check because json.Valid accepts invalid
// UTF-8, which Postgres text would refuse as INTERNAL. An escaped \u0000 is
// fine: it is plain ASCII in TEXT.
func validateDataJSON(s string) error {
	if len(s) > MaxDataJSONBytes || !utf8.ValidString(s) || !json.Valid([]byte(s)) {
		return ErrInvalidDataJSON
	}
	if trimmed := bytes.TrimSpace([]byte(s)); len(trimmed) == 0 || trimmed[0] != '{' {
		return ErrInvalidDataJSON
	}
	return nil
}

// NewEntry validates an event before RecordEntry stores it.
func NewEntry(eventID, eventType, producer string, occurredAt time.Time, dataJSON string) (Entry, error) {
	id, err := NormalizeID(eventID, ErrInvalidEventID)
	if err != nil {
		return Entry{}, err
	}
	if err := ValidateEventType(eventType); err != nil {
		return Entry{}, err
	}
	if err := validateProducer(producer); err != nil {
		return Entry{}, err
	}
	if occurredAt.IsZero() {
		return Entry{}, ErrInvalidOccurredAt
	}
	if err := validateDataJSON(dataJSON); err != nil {
		return Entry{}, err
	}
	return Entry{
		EventID:    id,
		EventType:  eventType,
		Producer:   producer,
		OccurredAt: occurredAt.UTC(),
		DataJSON:   dataJSON,
	}, nil
}

// ListFilter holds the optional ListAuditEntries filters. Empty EventType
// means all types; a nil time means no bound.
type ListFilter struct {
	EventType string
	Start     *time.Time // inclusive
	End       *time.Time // exclusive
}

// NewListFilter applies the contract rule that start must be before end
// when both are set. Equal times are rejected because the window would be
// empty by definition.
func NewListFilter(eventType string, start, end *time.Time) (ListFilter, error) {
	if eventType != "" {
		if err := ValidateEventType(eventType); err != nil {
			return ListFilter{}, err
		}
	}
	if start != nil && end != nil && !start.Before(*end) {
		return ListFilter{}, ErrInvalidTimeRange
	}
	return ListFilter{EventType: eventType, Start: utcPtr(start), End: utcPtr(end)}, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
