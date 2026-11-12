-- +goose Up
-- Keep the original protojson text and event identity unchanged for retries.
CREATE TABLE outgoing_events (
    event_id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (event_type = 'resume.uploaded'),
    payload TEXT NOT NULL CHECK (jsonb_typeof(payload::jsonb) = 'object'),
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    CHECK ((payload::jsonb ->> 'event_id') IS NOT DISTINCT FROM event_id::text),
    CHECK ((payload::jsonb ->> 'type') IS NOT DISTINCT FROM event_type)
);

CREATE INDEX outgoing_events_pending_idx ON outgoing_events (created_at, event_id)
    WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outgoing_events;
