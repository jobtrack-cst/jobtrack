-- +goose Up
-- Event IDs already applied by the job.closed consumer (JT-70).
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
