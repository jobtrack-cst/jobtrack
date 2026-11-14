-- +goose Up
CREATE TABLE processed_events (event_id UUID PRIMARY KEY);
CREATE TABLE outbox_events (
 sequence BIGSERIAL UNIQUE NOT NULL,
 event_id UUID PRIMARY KEY,
 routing_key TEXT NOT NULL CHECK (routing_key IN ('offer.made','offer.accepted','offer.declined')),
 payload TEXT NOT NULL CHECK (payload <> ''),
 created_at TIMESTAMPTZ NOT NULL,
 published_at TIMESTAMPTZ
);
CREATE INDEX outbox_events_unpublished ON outbox_events(sequence) WHERE published_at IS NULL;
-- +goose Down
DROP TABLE outbox_events;
DROP TABLE processed_events;
