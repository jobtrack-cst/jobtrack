-- +goose Up
-- JT-68 transactional outbox.
CREATE TABLE outbox_events (
    event_id     UUID PRIMARY KEY,
    routing_key  TEXT NOT NULL
        CHECK (routing_key IN ('job.published', 'job.updated', 'job.closed')),
    payload      TEXT NOT NULL CHECK (payload <> ''),
    created_at   TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_events_unpublished
    ON outbox_events (created_at, event_id) WHERE published_at IS NULL;

-- Idempotent consumer table (for moderation.job_removed)
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox_events;
