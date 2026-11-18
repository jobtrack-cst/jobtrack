-- +goose Up
CREATE TABLE processed_events (event_id UUID PRIMARY KEY);
CREATE TABLE outbox_events (
 sequence BIGSERIAL UNIQUE NOT NULL,
 event_id UUID PRIMARY KEY,
 routing_key TEXT NOT NULL CHECK (routing_key IN ('job_alert.matched')),
 payload TEXT NOT NULL CHECK (payload <> ''),
 created_at TIMESTAMPTZ NOT NULL,
 published_at TIMESTAMPTZ
);
CREATE INDEX outbox_events_unpublished ON outbox_events(sequence) WHERE published_at IS NULL;
CREATE TABLE matched_subscriptions (
 source_event_id UUID NOT NULL REFERENCES processed_events(event_id),
 alert_id UUID NOT NULL,
 job_id UUID NOT NULL,
 applicant_id UUID NOT NULL,
 PRIMARY KEY(source_event_id,alert_id)
);
-- +goose Down
DROP TABLE matched_subscriptions;
DROP TABLE outbox_events;
DROP TABLE processed_events;
