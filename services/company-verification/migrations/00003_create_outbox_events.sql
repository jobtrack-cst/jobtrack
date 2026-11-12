-- +goose Up
-- JT-56 transactional outbox. Decide stores the decision and its event in one
-- transaction; the relay publishes unpublished rows oldest first and then sets
-- published_at. Published rows are kept as a record of what was sent.
CREATE TABLE outbox_events (
    -- The envelope event_id.
    event_id     UUID PRIMARY KEY,
    routing_key  TEXT NOT NULL
        CHECK (routing_key IN ('employer.verified', 'employer.verification_rejected')),
    -- The protojson envelope exactly as pkg/events encodes it.
    payload      TEXT NOT NULL CHECK (payload <> ''),
    -- The decision time, which is also the envelope occurred_at.
    created_at   TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

-- The relay reads only unpublished rows, oldest first.
CREATE INDEX outbox_events_unpublished
    ON outbox_events (created_at, event_id) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outbox_events;
