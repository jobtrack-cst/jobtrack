-- +goose Up
CREATE TABLE processed_events (
 event_id UUID PRIMARY KEY,
 processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE outbox_events (
 outbox_id BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
 event_id UUID PRIMARY KEY,
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 published_at TIMESTAMPTZ
);
CREATE INDEX outbox_pending_idx ON outbox_events(outbox_id) WHERE published_at IS NULL;
-- +goose Down
DROP TABLE outbox_events;
DROP TABLE processed_events;
