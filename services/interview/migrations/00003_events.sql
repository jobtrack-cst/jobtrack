-- +goose Up
-- JT-80. Event IDs of handled application.submitted events, written in the
-- same transaction as the participants they add, so redelivery changes nothing.
CREATE TABLE processed_events (event_id UUID PRIMARY KEY);

-- interview.scheduled events stored in the ScheduleInterview transaction and
-- published by the relay afterwards. Published rows are kept as a record.
CREATE TABLE outbox_events (
    sequence     BIGSERIAL UNIQUE NOT NULL,
    event_id     UUID PRIMARY KEY,
    routing_key  TEXT NOT NULL CHECK (routing_key = 'interview.scheduled'),
    payload      TEXT NOT NULL CHECK (payload <> ''),
    created_at   TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);
CREATE INDEX outbox_events_unpublished ON outbox_events (sequence) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outbox_events;
DROP TABLE processed_events;
