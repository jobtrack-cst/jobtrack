-- +goose Up
-- Claimed and committed with the validation result, including ignored final results.
CREATE TABLE processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
